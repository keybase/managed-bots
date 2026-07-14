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
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"golang.org/x/oauth2"
)

type OAuthRequiredError struct{}

func (e OAuthRequiredError) Error() string {
	return "OAuth is required for this, permission requested."
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
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) {
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
	// renew token
	if token.Expiry.Before(time.Now()) {
		newToken, err := config.TokenSource(ctx, token).Token()
		if err != nil {
			return nil, fmt.Errorf("unable to renew token: %s", err)
		}
		err = storage.PutToken(ctx, tokenIdentifier, newToken)
		if err != nil {
			return nil, fmt.Errorf("unable to update token: %s", err)
		}
		token = newToken
	}

	return config.Client(ctx, token), nil
}
