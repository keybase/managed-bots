package base

import (
	"context"
	"crypto/subtle"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Each flow gets its own cookie (the state is base64url, which is valid in a
// cookie name), so starting a second link in the same browser doesn't break
// the first.
const oauthStateCookiePrefix = "oauth_state_"

// OAuthStartURL is the bot's start page for state. It lives under the
// provider callback URL, so the start page, the callback and the state cookie
// share the path the browser actually sees.
func OAuthStartURL(config *oauth2.Config, state string) string {
	return strings.TrimSuffix(config.RedirectURL, "/") + "/start?" + url.Values{"state": {state}}.Encode()
}

// oauthCookiePath is the browser-facing path of the provider callback.
func oauthCookiePath(config *oauth2.Config) string {
	u, err := url.Parse(config.RedirectURL)
	if err != nil || u.Path == "" {
		return "/"
	}
	return strings.TrimSuffix(u.Path, "/")
}

// setOAuthStateCookie binds this browser to state. The callback only accepts
// state from a browser that went through the start page. SameSite=Lax (not
// Strict) is required since the provider redirects back cross-site.
func setOAuthStateCookie(w http.ResponseWriter, path, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookiePrefix + state,
		Value:    state,
		Path:     path,
		MaxAge:   int(OAuthStateTTL / time.Second),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// hasOAuthStateCookie reports whether the request carries the cookie set by
// setOAuthStateCookie for state.
func hasOAuthStateCookie(r *http.Request, state string) bool {
	if state == "" {
		return false
	}
	cookie, err := r.Cookie(oauthStateCookiePrefix + state)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) == 1
}

// setOAuthPageHeaders keeps the OAuth pages from being framed or cached.
func setOAuthPageHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

// makeOAuthStartBody is the page shown before redirecting to the provider. It
// names the Keybase account the link is for, so someone who was forwarded the
// link can tell it isn't theirs.
func makeOAuthStartBody(botUsername, requester, team, authURL string) string {
	target := fmt.Sprintf("Keybase user <b>@%s</b>", html.EscapeString(requester))
	if team != "" && team != requester {
		target += fmt.Sprintf(" for the team <b>%s</b> (everyone in it will be able to use the account)",
			html.EscapeString(team))
	}
	return fmt.Sprintf(`<div>You're about to connect the account you sign into next to %s, through <b>@%s</b>.</div>
		<div class="msg"><b>If you are not @%s, or someone sent you this link, stop and close this page.</b></div>
		<div class="msg"><a href="%s">Continue</a></div>`,
		target, html.EscapeString(botUsername), html.EscapeString(requester), html.EscapeString(authURL))
}

// makeOAuthConfirmBody is the page shown after the provider redirects back. The
// link only completes once the requester sends confirmCmd to botUsername from
// Keybase. The bot's name links to a direct message with it in the Keybase app
// (keybase://chat/<username>); the app can't prefill the message text.
func makeOAuthConfirmBody(botUsername, requester, confirmCmd string) string {
	chatLink := "keybase://chat/" + url.PathEscape(botUsername)
	return fmt.Sprintf(`<div class="success">Almost done!</div>
		<div>To finish, send this message to <a href="%s"><b>@%s</b></a> in Keybase from <b>@%s</b>'s account:</div>
		<div class="msg"><code>%s</code></div>
		<div class="msg"><a href="%s">Open a chat with @%s in the Keybase app</a></div>
		<div class="msg">This code expires in %d minutes. <b>Never send it to anyone else.</b>
		Whoever sends it will get access to the account you just authorized.</div>`,
		html.EscapeString(chatLink), html.EscapeString(botUsername), html.EscapeString(requester),
		html.EscapeString(confirmCmd), html.EscapeString(chatLink), html.EscapeString(botUsername),
		int(OAuthStateTTL/time.Minute))
}

const oauthDoneBody = `<div class="success">Success!</div>
	<div>You can now close this page and return to the Keybase app.</div>`

// OAuthFlowStorage is what OAuthFlow needs from a bot's oauth_state table.
// OAuthStateDB provides everything but GetState.
type OAuthFlowStorage interface {
	// GetState returns nil if state doesn't exist or has expired.
	GetState(ctx context.Context, state string) (*OAuthRequest, error)
	SetPendingToken(ctx context.Context, state, codeHash, pendingToken string) (bool, error)
	ReissueConfirmCode(ctx context.Context, state, codeHash string) (bool, error)
}

// OAuthFlow serves the browser side of linking an account:
//
//  1. HandleStart (<callback>/start) names the Keybase account being linked,
//     binds the browser to the state with a cookie, and links to the provider.
//  2. HandleCallback exchanges the code, holds the token as pending, and
//     shows a confirmation code. Reloading it in the same browser shows a new
//     code.
//  3. The requester sends "<confirmCmd> <code>" from Keybase, which the bot
//     passes to ConfirmPendingOAuth (or ConfirmOAuth) to save the token.
//
// Step 3 is what keeps a forwarded link from attaching someone else's account
// to the requester: whoever authorized at the provider has to hand the code
// back to them.
type OAuthFlow struct {
	*DebugOutput
	oauth        *oauth2.Config
	authCodeOpts []oauth2.AuthCodeOption
	storage      OAuthFlowStorage
	confirmCmd   string
	botUsername  string
	cookiePath   string
	htmlTitle    string
	htmlLogoSrc  string
}

// NewOAuthFlow returns the handlers for a flow whose provider callback
// (oauth.RedirectURL) is served at callbackPath on this server. Start links
// are built from RedirectURL (see OAuthStartURL), so the start handler must be
// registered at callbackPath + "/start".
func NewOAuthFlow(
	debug *DebugOutput,
	oauth *oauth2.Config,
	storage OAuthFlowStorage,
	confirmCmd, botUsername, htmlTitle, htmlLogoSrc, callbackPath string,
	authCodeOpts ...oauth2.AuthCodeOption,
) *OAuthFlow {
	f := &OAuthFlow{
		DebugOutput:  debug,
		oauth:        oauth,
		authCodeOpts: append([]oauth2.AuthCodeOption{oauth2.ApprovalForce}, authCodeOpts...),
		storage:      storage,
		confirmCmd:   confirmCmd,
		botUsername:  botUsername,
		cookiePath:   oauthCookiePath(oauth),
		htmlTitle:    htmlTitle,
		htmlLogoSrc:  htmlLogoSrc,
	}
	if f.cookiePath != callbackPath {
		// Behind a proxy that rewrites paths this can be fine, but otherwise
		// start links won't reach this server.
		f.Errorf("NewOAuthFlow: RedirectURL path %q differs from the callback path %q; check start links reach %s/start",
			f.cookiePath, callbackPath, callbackPath)
	}
	return f
}

func (f *OAuthFlow) HandleStart(w http.ResponseWriter, r *http.Request) {
	setOAuthPageHeaders(w)
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	state := r.URL.Query().Get("state")
	req, err := f.storage.GetState(r.Context(), state)
	if err != nil {
		f.Errorf("HandleStart: could not get state: %v", err)
		f.showError(w)
		return
	} else if req == nil || req.IsComplete || req.IsPending {
		f.showError(w)
		return
	}

	setOAuthStateCookie(w, f.cookiePath, state)
	authURL := f.oauth.AuthCodeURL(state, f.authCodeOpts...)
	f.write(w, "authorize", makeOAuthStartBody(f.botUsername, req.Requester, req.TokenIdentifier, authURL))
}

func (f *OAuthFlow) HandleCallback(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			LogOAuthError(f.DebugOutput, "HandleCallback", err)
			f.showError(w)
		}
	}()

	if r.URL == nil {
		err = fmt.Errorf("r.URL == nil")
		return
	}

	setOAuthPageHeaders(w)
	query := r.URL.Query()
	state := query.Get("state")
	// WithoutCancel: the browser may close after the redirect; the DB writes
	// must finish regardless.
	ctx := context.WithoutCancel(r.Context())

	req, err := f.storage.GetState(ctx, state)
	if err != nil {
		err = fmt.Errorf("could not get state: %v", err)
		return
	} else if req == nil {
		// unknown or expired
		f.showError(w)
		return
	} else if req.IsComplete {
		f.write(w, "success", oauthDoneBody)
		return
	}

	if !hasOAuthStateCookie(r, state) {
		f.Debug("HandleCallback: browser did not start this authorization")
		f.showError(w)
		return
	}

	confirmCode, codeHash, err := MakeConfirmCode()
	if err != nil {
		return
	}

	var ok bool
	if req.IsPending {
		// A reload in the browser that authorized: the provider code was
		// already exchanged, so show a new confirmation code instead.
		ok, err = f.storage.ReissueConfirmCode(ctx, state, codeHash)
	} else {
		code := query.Get("code")
		if code == "" {
			// no code is provided
			f.showError(w)
			return
		}
		var token *oauth2.Token
		if token, err = f.oauth.Exchange(ctx, code); err != nil {
			return
		}
		var pendingToken string
		if pendingToken, err = MarshalPendingToken(token); err != nil {
			return
		}
		ok, err = f.storage.SetPendingToken(ctx, state, codeHash, pendingToken)
	}
	if err != nil {
		return
	} else if !ok {
		f.showError(w)
		return
	}

	f.write(w, "confirm", makeOAuthConfirmBody(f.botUsername, req.Requester, f.confirmCmd+" "+confirmCode))
}

func (f *OAuthFlow) showError(w http.ResponseWriter) {
	f.write(w, "error", "Unable to complete request, please try running the bot command again!")
}

func (f *OAuthFlow) write(w http.ResponseWriter, title, body string) {
	if _, err := w.Write(MakeOAuthHTML(f.htmlTitle, title, body, f.htmlLogoSrc)); err != nil {
		f.Errorf("OAuthFlow: unable to write: %v", err)
	}
}
