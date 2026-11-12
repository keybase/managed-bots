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
	if retr, ok := errors.AsType[*oauth2.RetrieveError](err); ok {
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
	if retr, ok := errors.AsType[*oauth2.RetrieveError](err); ok {
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

	PutState(ctx context.Context, state string, req *OAuthRequest) error

	OAuthFlowStorage
	PendingOAuthStorage
}

// OAuthHTTPSrv serves the OAuthFlow (see it for the steps) for bots that get
// tokens through GetOAuthClient. The bot passes its messages to
// HandleConfirmCmd.
type OAuthHTTPSrv struct {
	*HTTPSrv
	htmlLogoB64 string
}

// NewOAuthHTTPSrv registers the OAuth handlers under urlPrefix. confirmCmd is
// the chat command (e.g. "!github confirm") the bot handles with
// HandleConfirmCmd.
func NewOAuthHTTPSrv(
	stats *StatsRegistry,
	kbc *kbchat.API,
	debugConfig *ChatDebugOutputConfig,
	oauth *oauth2.Config,
	storage OAuthStorage,
	confirmCmd string,
	htmlTitle string,
	htmlLogoB64 string,
	urlPrefix string,
	authCodeOpts ...oauth2.AuthCodeOption,
) *OAuthHTTPSrv {
	o := &OAuthHTTPSrv{
		HTTPSrv:     NewHTTPSrv(stats, debugConfig),
		htmlLogoB64: htmlLogoB64,
	}
	callbackPath := urlPrefix + "/oauth"
	htmlLogoSrc := urlPrefix + "/image/logo"
	flow := NewOAuthFlow(o.DebugOutput, oauth, storage, confirmCmd, kbc.GetUsername(), htmlTitle,
		htmlLogoSrc, callbackPath, authCodeOpts...)
	http.HandleFunc(callbackPath, flow.HandleCallback)
	http.HandleFunc(callbackPath+"/start", flow.HandleStart)
	http.HandleFunc(htmlLogoSrc, o.logoHandler)
	return o
}

func getCallbackMsg(kbc *kbchat.API, req OAuthRequest) (res chat1.MsgSummary, err error) {
	msgs, err := kbc.GetMessagesByConvID(req.ConvID, []chat1.MessageID{req.MsgID})
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

// HandleConfirmCmd reports whether msg is "<confirmCmd> <code>" and handles it
// if so: when the code matches one of the sender's pending links, the token is
// stored and callback is run with the message that originally required auth.
// Failures are reported to the user here, so err only comes from callback.
func HandleConfirmCmd(
	ctx context.Context,
	debug *DebugOutput,
	kbc *kbchat.API,
	storage OAuthStorage,
	msg chat1.MsgSummary,
	confirmCmd string,
	callback func(ctx context.Context, msg chat1.MsgSummary, identifier string) error,
) (handled bool, err error) {
	if msg.Content.Text == nil {
		return false, nil
	}
	code, ok := ParseConfirmCmd(msg.Content.Text.Body, confirmCmd)
	if !ok {
		return false, nil
	} else if code == "" {
		debug.ChatEcho(msg.ConvID, "%s", ConfirmUsageMsg(confirmCmd))
		return true, nil
	}

	var req *OAuthRequest
	confirmed, err := ConfirmPendingOAuth(ctx, storage, msg.Sender.Username, code,
		func(ctx context.Context, state string, token *oauth2.Token) (err error) {
			if req, err = storage.GetState(ctx, state); err != nil {
				return err
			} else if req == nil {
				return fmt.Errorf("state expired before the token was saved")
			}
			return storage.PutToken(ctx, req.TokenIdentifier, token)
		})
	if err != nil {
		debug.Errorf("HandleConfirmCmd: %v", err)
		debug.ChatEcho(msg.ConvID, "%s", ConfirmSaveFailedMsg)
		return true, nil
	} else if !confirmed {
		debug.ChatEcho(msg.ConvID, "%s", ConfirmNoMatchMsg("Run your command again to get a new link."))
		return true, nil
	}

	callbackMsg, err := getCallbackMsg(kbc, *req)
	if err != nil {
		debug.Errorf("HandleConfirmCmd: unable to get callback message: %v", err)
		debug.ChatEcho(msg.ConvID, "You're authorized, but I couldn't find the command you sent before. Please send it again.")
		return true, nil
	}
	return true, callback(ctx, callbackMsg, req.TokenIdentifier)
}

func (o *OAuthHTTPSrv) logoHandler(w http.ResponseWriter, _ *http.Request) {
	dat, _ := base64.StdEncoding.DecodeString(o.htmlLogoB64)
	if _, err := io.Copy(w, bytes.NewBuffer(dat)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

type OAuthRequest struct {
	IsComplete bool
	// IsPending is set once the provider redirected back and the token is
	// waiting for the requester's confirmation.
	IsPending       bool
	TokenIdentifier string
	// Requester is the Keybase user who asked for the link. TokenIdentifier
	// may be a team name instead.
	Requester string
	ConvID    chat1.ConvIDStr
	MsgID     chat1.MessageID
}

type GetOAuthOpts struct {
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
			Requester:       callbackMsg.Sender.Username,
			ConvID:          callbackMsg.ConvID,
			MsgID:           callbackMsg.Id,
		}); err != nil {
			return nil, err
		}

		// Send the bot's start page, not the provider URL: the start page binds
		// the browser to the state (see OAuthFlow).
		authURL := OAuthStartURL(config, state)
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
