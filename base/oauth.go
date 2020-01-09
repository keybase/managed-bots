package base

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"golang.org/x/oauth2"
)

type OAuthStorage interface {
	GetToken(identifier string) (*oauth2.Token, error)
	PutToken(identifier string, token *oauth2.Token) error
	DeleteToken(identifier string) error
}

type OAuthHTTPSrv struct {
	*HTTPSrv
	oauth       *oauth2.Config
	requests    *OAuthRequests
	storage     OAuthStorage
	callback    func(chat1.MsgSummary) error
	htmlTitle   string
	htmlLogoB64 string
	htmlLogoSrc string
}

func NewOAuthHTTPSrv(
	kbc *kbchat.API,
	oauth *oauth2.Config,
	requests *OAuthRequests,
	storage OAuthStorage,
	callback func(chat1.MsgSummary) error,
	htmlTitle string,
	htmlLogoB64 string,
	urlPrefix string,
) *OAuthHTTPSrv {
	o := &OAuthHTTPSrv{
		oauth:       oauth,
		requests:    requests,
		storage:     storage,
		callback:    callback,
		htmlTitle:   htmlTitle,
		htmlLogoB64: htmlLogoB64,
		htmlLogoSrc: urlPrefix + "/image/logo",
	}
	o.HTTPSrv = NewHTTPSrv(kbc)
	http.HandleFunc(urlPrefix+"/oauth", o.oauthHandler)
	http.HandleFunc(o.htmlLogoSrc, o.logoHandler)
	return o
}

func (o *OAuthHTTPSrv) oauthHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			o.Debug("oauthHandler: %v", err)
			if _, err := w.Write(MakeOAuthHTML(o.htmlTitle, "error", "Unable to complete request, please try again!", o.htmlLogoSrc)); err != nil {
				o.Debug("oauthHandler: unable to write: %v", err)
			}
		}
	}()

	if r.URL == nil {
		err = fmt.Errorf("r.URL == nil")
		return
	}

	query := r.URL.Query()
	state := query.Get("state")

	req, ok := o.requests.Get(state)
	o.requests.Delete(state)
	if !ok {
		err = fmt.Errorf("state %q not found %v", state, o.requests)
		return
	}

	code := query.Get("code")
	token, err := o.oauth.Exchange(context.TODO(), code)
	if err != nil {
		return
	}

	if err = o.storage.PutToken(req.tokenIdentifier, token); err != nil {
		return
	}

	if err = o.callback(req.callbackMsg); err != nil {
		return
	}

	if _, err := w.Write(MakeOAuthHTML(o.htmlTitle, "success", "Success! You can now close this page and return to the Keybase app.", o.htmlLogoSrc)); err != nil {
		o.Debug("oauthHandler: unable to write: %v", err)
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
	tokenIdentifier string
	callbackMsg     chat1.MsgSummary
}

type OAuthRequests struct {
	sync.Map
}

func (r *OAuthRequests) Get(state string) (req *OAuthRequest, ok bool) {
	val, ok := r.Map.Load(state)
	if ok {
		return val.(*OAuthRequest), ok
	} else {
		return nil, ok
	}
}

func (r *OAuthRequests) Set(state string, req *OAuthRequest) {
	r.Map.Store(state, req)
}

func (r *OAuthRequests) Delete(state string) {
	r.Map.Delete(state)
}

type GetOAuthOpts struct {
	// all non-admin users can also authenticate (default: false)
	AllowNonAdminForTeamAuth bool
	// set the OAuth2 OfflineAccessType (default: false)
	OAuthOfflineAccessType bool
}

func GetOAuthClient(
	tokenIdentifier string,
	callbackMsg chat1.MsgSummary,
	kbc *kbchat.API,
	requests *OAuthRequests,
	config *oauth2.Config,
	storage OAuthStorage,
	authMessageTemplate string,
	opts GetOAuthOpts,
) (*http.Client, error) {
	token, err := storage.GetToken(tokenIdentifier)
	if err != nil {
		return nil, err
	}

	// we need to request new authorization
	if token == nil {
		// if required, check if the user is an admin before executing auth
		if !opts.AllowNonAdminForTeamAuth {
			isAdmin, err := IsAdmin(kbc, callbackMsg)
			if err != nil {
				return nil, err
			}
			if !isAdmin {
				_, err = kbc.SendMessageByConvID(callbackMsg.ConvID, "You have must be an admin to authorize me for a team!")
				return nil, err
			}
		}

		state, err := MakeRequestID()
		if err != nil {
			return nil, err
		}
		requests.Set(state, &OAuthRequest{tokenIdentifier, callbackMsg})

		oauthOpts := []oauth2.AuthCodeOption{oauth2.ApprovalForce}
		if opts.OAuthOfflineAccessType {
			oauthOpts = append(oauthOpts, oauth2.AccessTypeOffline)
		}
		authURL := config.AuthCodeURL(state, oauthOpts...)
		// strip protocol to skip unfurl prompt
		authURL = strings.TrimPrefix(authURL, "https://")
		_, err = kbc.SendMessageByTlfName(callbackMsg.Sender.Username, authMessageTemplate, authURL)

		// If we are in a 1-1 conv directly or as a bot user with the sender, skip this message.
		if callbackMsg.Channel.MembersType == "team" || !(callbackMsg.Sender.Username == callbackMsg.Channel.Name ||
			len(strings.Split(callbackMsg.Channel.Name, ",")) == 2) {
			_, err = kbc.SendMessageByConvID(callbackMsg.ConvID,
				"OK! I've sent a message to @%s to authorize me.", callbackMsg.Sender.Username)
		}

		return nil, err
	}

	return config.Client(context.Background(), token), nil
}
