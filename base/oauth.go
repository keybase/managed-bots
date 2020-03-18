package base

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"golang.org/x/oauth2"
)

type OAuthRequiredError struct{}

func (e OAuthRequiredError) Error() string {
	return "OAuth is required for this, permission requested."
}

type OAuthStorage interface {
	GetToken(identifier string) (*oauth2.Token, error)
	PutToken(identifier string, token *oauth2.Token) error
	DeleteToken(identifier string) error

	GetState(state string) (*OAuthRequest, error)
	PutState(state string, req *OAuthRequest) error
	CompleteState(state string) error
}

type OAuthHTTPSrv struct {
	*HTTPSrv
	kbc         *kbchat.API
	oauth       *oauth2.Config
	storage     OAuthStorage
	callback    func(msg chat1.MsgSummary, identifier string) error
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
	callback func(msg chat1.MsgSummary, identifier string) error,
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

func (o *OAuthHTTPSrv) oauthHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			o.Errorf("oauthHandler: %v", err)
			o.showOAuthError(w)
		}
	}()

	if r.URL == nil {
		err = fmt.Errorf("r.URL == nil")
		return
	}

	query := r.URL.Query()
	state := query.Get("state")

	req, err := o.storage.GetState(state)
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
	token, err := o.oauth.Exchange(context.TODO(), code)
	if err != nil {
		return
	}

	if err = o.storage.PutToken(req.TokenIdentifier, token); err != nil {
		return
	}
	if err = o.storage.CompleteState(state); err != nil {
		return
	}
	callbackMsg, err := o.getCallbackMsg(*req)
	if err != nil {
		return
	}

	if err = o.callback(callbackMsg, req.TokenIdentifier); err != nil {
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

func (o *OAuthHTTPSrv) logoHandler(w http.ResponseWriter, r *http.Request) {
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
	tokenIdentifier string,
	callbackMsg chat1.MsgSummary,
	kbc *kbchat.API,
	config *oauth2.Config,
	storage OAuthStorage,
	opts GetOAuthOpts,
) (*http.Client, error) {
	token, err := storage.GetToken(tokenIdentifier)
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
		if err := storage.PutState(state, &OAuthRequest{
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
	} else {
		// renew token
		newToken, err := config.TokenSource(context.Background(), token).Token()
		if err != nil {
			return nil, fmt.Errorf("unable to renew token: %s", err)
		}
		if newToken.AccessToken != token.AccessToken {
			err = storage.PutToken(tokenIdentifier, newToken)
			if err != nil {
				return nil, fmt.Errorf("unable to update token: %s", err)
			}
			token = newToken
		}
	}

	return config.Client(context.Background(), token), nil
}
