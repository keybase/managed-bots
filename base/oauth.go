package base

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"golang.org/x/oauth2"
)

type OAuthRequiredError struct{}

func (e OAuthRequiredError) Error() string {
	return "OAuth is required for this, permission requested."
}

// ShouldRetryAuth reports whether err means the user's OAuth credentials are
// permanently unusable and should be deleted: invalid_grant, invalid_token,
// a missing refresh token, or a Google Workspace Account Restricted refresh
// failure. Transient token-fetch failures (network, 5xx) and a bare
// access_not_configured (API not enabled on the Cloud project) are not.
func ShouldRetryAuth(err error) bool {
	if err == nil {
		return false
	}
	if IsOAuthAccountRestricted(err) {
		return true
	}
	var retr *oauth2.RetrieveError
	if errors.As(err, &retr) {
		switch strings.ToLower(retr.ErrorCode) {
		case "invalid_grant", "invalid_token":
			return true
		}
		body := string(retr.Body)
		return strings.Contains(body, "invalid_grant") ||
			strings.Contains(body, "invalid_token")
	}
	msg := err.Error()
	return strings.Contains(msg, "invalid_grant") ||
		strings.Contains(msg, "token expired and refresh token is not set")
}

// IsOAuthAccountRestricted reports whether Google Workspace has blocked this
// OAuth client for the user (token refresh returns access_not_configured /
// Account Restricted). This is per-account admin policy, not a missing API on
// the Cloud project.
func IsOAuthAccountRestricted(err error) bool {
	if err == nil {
		return false
	}
	restricted := func(s string) bool {
		s = strings.ToLower(s)
		return strings.Contains(s, "account restricted") ||
			strings.Contains(s, "servicenotallowed")
	}
	var retr *oauth2.RetrieveError
	if errors.As(err, &retr) {
		code := strings.ToLower(retr.ErrorCode)
		body := strings.ToLower(string(retr.Body))
		if code != "access_not_configured" && !strings.Contains(body, "access_not_configured") {
			return false
		}
		return restricted(retr.ErrorDescription) ||
			restricted(retr.ErrorURI) ||
			restricted(body)
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access_not_configured") && restricted(msg)
}

type OAuthStorage interface {
	GetToken(ctx context.Context, identifier string) (*oauth2.Token, error)
	PutToken(ctx context.Context, identifier string, token *oauth2.Token) error
	DeleteToken(ctx context.Context, identifier string) error

	GetState(ctx context.Context, state string) (*OAuthRequest, error)
	PutState(ctx context.Context, state string, req *OAuthRequest) error
	CompleteState(ctx context.Context, state string) error
}

type OAuthHTTPSrv struct {
	*HTTPSrv
	kbc         *kbchat.API
	oauth       *oauth2.Config
	storage     OAuthStorage
	callback    func(ctx context.Context, msg chat1.MsgSummary, identifier string) error
	htmlTitle   string
	htmlLogoB64 string
	htmlLogoSrc string
}

func NewOAuthHTTPSrv(
	stats *StatsRegistry,
	kbc *kbchat.API,
	debugConfig *ChatDebugOutputConfig,
	oauth *oauth2.Config,
	storage OAuthStorage,
	callback func(ctx context.Context, msg chat1.MsgSummary, identifier string) error,
	htmlTitle string,
	htmlLogoB64 string,
	urlPrefix string,
) *OAuthHTTPSrv {
	o := &OAuthHTTPSrv{
		kbc:         kbc,
		oauth:       oauth,
		storage:     storage,
		callback:    callback,
		htmlTitle:   htmlTitle,
		htmlLogoB64: htmlLogoB64,
		htmlLogoSrc: urlPrefix + "/image/logo",
	}
	o.HTTPSrv = NewHTTPSrv(stats, debugConfig)
	http.HandleFunc(urlPrefix+"/oauth", o.oauthHandler)
	http.HandleFunc(o.htmlLogoSrc, o.logoHandler)
	return o
}

func (o *OAuthHTTPSrv) getCallbackMsg(req OAuthRequest) (res chat1.MsgSummary, err error) {
	msgs, err := o.kbc.GetMessagesByConvID(req.ConvID, []chat1.MessageID{req.MsgID})
	if err != nil {
		return res, err
	}
	if len(msgs) != 1 {
		return res, fmt.Errorf("Unable to find msg %d in %s, got back %d messages",
			req.MsgID, req.ConvID, len(msgs))
	}
	msg := msgs[0]
	if msg.Error != nil || msg.Msg == nil {
		return res, fmt.Errorf("invalid callback message %v", msg)
	}
	return *msg.Msg, nil
}

// LogOAuthError logs an OAuth error, scrubbing any raw token-endpoint response
// body. ErrorCode and ErrorDescription from structured OAuth errors are retained.
func LogOAuthError(debug *DebugOutput, context string, err error) {
	if retrieveErr, ok := errors.AsType[*oauth2.RetrieveError](err); ok {
		statusCode := 0
		if retrieveErr.Response != nil {
			statusCode = retrieveErr.Response.StatusCode
		}
		debug.Errorf("%s: token exchange failed (status %d, %q: %s)", context, statusCode, retrieveErr.ErrorCode, retrieveErr.ErrorDescription)
	} else {
		debug.Errorf("%s: %v", context, err)
	}
}

func (o *OAuthHTTPSrv) oauthHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			LogOAuthError(o.DebugOutput, "oauthHandler", err)
			o.showOAuthError(w)
		}
	}()

	if r.URL == nil {
		err = fmt.Errorf("r.URL == nil")
		return
	}

	query := r.URL.Query()
	state := query.Get("state")
	// WithoutCancel: the browser may close after the redirect; DB writes and the
	// HandleAuth callback (which sends Keybase messages) must finish regardless.
	ctx := context.WithoutCancel(r.Context())

	req, err := o.storage.GetState(ctx, state)
	if err != nil {
		err = fmt.Errorf("could not get state %q: %v", state, err)
		return
	} else if req == nil {
		// no state is found
		o.showOAuthError(w)
		return
	}

	if req.IsComplete {
		_, err = w.Write(MakeOAuthHTML(o.htmlTitle, "success",
			`<div class="success"> Success! </div>
		<div>You can now close this page and return to the Keybase app.</div>`,
			o.htmlLogoSrc))
		if err != nil {
			o.Errorf("oauthHandler: unable to write: %v", err)
		}
		return
	}

	code := query.Get("code")
	if code == "" {
		// no code is provided
		o.showOAuthError(w)
		return
	}
	token, err := o.oauth.Exchange(ctx, code)
	if err != nil {
		return
	}

	if err = o.storage.PutToken(ctx, req.TokenIdentifier, token); err != nil {
		return
	}
	if err = o.storage.CompleteState(ctx, state); err != nil {
		return
	}
	callbackMsg, err := o.getCallbackMsg(*req)
	if err != nil {
		return
	}

	if err = o.callback(ctx, callbackMsg, req.TokenIdentifier); err != nil {
		return
	}

	_, err = w.Write(MakeOAuthHTML(o.htmlTitle, "success",
		`<div class="success"> Success! </div>
		<div>You can now close this page and return to the Keybase app.</div>`,
		o.htmlLogoSrc))
	if err != nil {
		o.Errorf("oauthHandler: unable to write: %v", err)
	}
}

func (o *OAuthHTTPSrv) showOAuthError(w http.ResponseWriter) {
	if _, err := w.Write(MakeOAuthHTML(o.htmlTitle, "error",
		"Unable to complete request, please try running the bot command again!", o.htmlLogoSrc)); err != nil {
		o.Errorf("oauthHandler: unable to write: %v", err)
	}
}

func (o *OAuthHTTPSrv) logoHandler(w http.ResponseWriter, _ *http.Request) {
	dat, _ := base64.StdEncoding.DecodeString(o.htmlLogoB64)
	if _, err := io.Copy(w, bytes.NewBuffer(dat)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

type OAuthRequest struct {
	IsComplete      bool
	TokenIdentifier string
	ConvID          chat1.ConvIDStr
	MsgID           chat1.MessageID
}

type GetOAuthOpts struct {
	// set the OAuth2 OfflineAccessType (default: false)
	OAuthOfflineAccessType bool
	// template for the auth message (default: "Visit %s\n to authorize me.")
	AuthMessageTemplate string
	// optional callback which constructs and sends auth URL (default: disabled)
	AuthURLCallback func(authUrl string) error
}

func GetOAuthClient(
	ctx context.Context,
	tokenIdentifier string,
	callbackMsg chat1.MsgSummary,
	kbc *kbchat.API,
	config *oauth2.Config,
	storage OAuthStorage,
	opts GetOAuthOpts,
) (*http.Client, error) {
	token, err := storage.GetToken(ctx, tokenIdentifier)
	if err != nil {
		return nil, err
	}

	// we need to request new authorization
	if token == nil {
		isAllowed, err := IsAtLeastWriter(kbc, callbackMsg.Sender.Username, callbackMsg.Channel)
		if err != nil {
			return nil, err
		}
		if !isAllowed {
			_, err = kbc.SendMessageByConvID(callbackMsg.ConvID, "You must be at least a writer to authorize me for a team!")
			return nil, err
		}

		state, err := MakeRequestID()
		if err != nil {
			return nil, err
		}
		if err := storage.PutState(ctx, state, &OAuthRequest{
			TokenIdentifier: tokenIdentifier,
			ConvID:          callbackMsg.ConvID,
			MsgID:           callbackMsg.Id,
		}); err != nil {
			return nil, err
		}

		oauthOpts := []oauth2.AuthCodeOption{oauth2.ApprovalForce}
		if opts.OAuthOfflineAccessType {
			oauthOpts = append(oauthOpts, oauth2.AccessTypeOffline)
		}
		authURL := config.AuthCodeURL(state, oauthOpts...)
		// strip protocol to skip unfurl prompt
		authURL = strings.TrimPrefix(authURL, "https://")
		if opts.AuthURLCallback != nil {
			err = opts.AuthURLCallback(authURL)
		} else {
			authMessageTemplate := opts.AuthMessageTemplate
			if authMessageTemplate == "" {
				authMessageTemplate = "Visit %s\n to authorize me."
			}
			_, err = kbc.SendMessageByTlfName(callbackMsg.Sender.Username, authMessageTemplate, authURL)
		}
		if err != nil {
			return nil, fmt.Errorf("error sending message: %s", err)
		}

		// If we are in a 1-1 conv directly or as a bot user with the sender, skip this message.
		if !IsDirectPrivateMessage(kbc.GetUsername(), callbackMsg.Sender.Username, callbackMsg.Channel) {
			_, err = kbc.SendMessageByConvID(callbackMsg.ConvID,
				"OK! I've sent a message to @%s to authorize me.", callbackMsg.Sender.Username)
			if err != nil {
				return nil, fmt.Errorf("error sending message: %s", err)
			}
		}

		return nil, OAuthRequiredError{}
	}

	src := PersistTokenSource(ctx, token, ConfigTokenSource(ctx, config, token), func(ctx context.Context, tok *oauth2.Token) error {
		return storage.PutToken(ctx, tokenIdentifier, tok)
	})
	if _, err := src.Token(); err != nil {
		return nil, fmt.Errorf("unable to renew token: %w", err)
	}
	return oauth2.NewClient(ctx, src), nil
}

// ConfigTokenSource is config.TokenSource, except tokens with a zero Expiry
// and a refresh token are treated as expired. oauth2.Token.Valid treats a
// zero Expiry as never-expired, which would skip refresh forever.
func ConfigTokenSource(ctx context.Context, config *oauth2.Config, token *oauth2.Token) oauth2.TokenSource {
	if token != nil && token.Expiry.IsZero() && token.RefreshToken != "" {
		cp := *token
		cp.Expiry = time.Now().Add(-time.Minute)
		token = &cp
	}
	return config.TokenSource(ctx, token)
}

// PersistTokenSource wraps src and writes the token whenever AccessToken,
// RefreshToken, or Expiry changes (including refresh-token rotation).
func PersistTokenSource(ctx context.Context, token *oauth2.Token, src oauth2.TokenSource, put func(context.Context, *oauth2.Token) error) oauth2.TokenSource {
	// Token() is invoked on later HTTP refreshes; the caller context may
	// already be done by then, so persist independently of it.
	return &persistTokenSource{ctx: context.WithoutCancel(ctx), token: token, src: src, put: put}
}

type persistTokenSource struct {
	ctx   context.Context
	token *oauth2.Token
	src   oauth2.TokenSource
	put   func(context.Context, *oauth2.Token) error
	mu    sync.Mutex
}

func (s *persistTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tok, err := s.src.Token()
	if err != nil {
		return nil, err
	}
	if tok.AccessToken != s.token.AccessToken ||
		tok.RefreshToken != s.token.RefreshToken ||
		!tok.Expiry.Equal(s.token.Expiry) {
		*s.token = *tok
		if err := s.put(s.ctx, tok); err != nil {
			return nil, fmt.Errorf("unable to update token: %w", err)
		}
	}
	return tok, nil
}
