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

	originatingMsg, ok := o.requests.Get(state)
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

	if err = o.storage.PutToken(IdentifierFromMsg(originatingMsg), token); err != nil {
		return
	}

	if err = o.callback(originatingMsg); err != nil {
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

type OAuthRequests struct {
	sync.Mutex

	requests map[string]chat1.MsgSummary
}

func NewOAuthRequests() *OAuthRequests {
	return &OAuthRequests{
		requests: make(map[string]chat1.MsgSummary),
	}
}

func (r *OAuthRequests) Get(state string) (msg chat1.MsgSummary, ok bool) {
	defer r.Unlock()
	r.Lock()
	msg, ok = r.requests[state]
	return msg, ok
}

func (r *OAuthRequests) Set(state string, msg chat1.MsgSummary) {
	r.Lock()
	r.requests[state] = msg
	r.Unlock()
}

func (r *OAuthRequests) Delete(state string) {
	r.Lock()
	delete(r.requests, state)
	r.Unlock()
}

func GetOAuthClient(
	msg chat1.MsgSummary,
	kbc *kbchat.API,
	requests *OAuthRequests,
	config *oauth2.Config,
	storage OAuthStorage,
	authMessageTemplate string,
) (*http.Client, error) {
	identifier := IdentifierFromMsg(msg)
	token, err := storage.GetToken(identifier)
	if err != nil {
		return nil, err
	}

	// we need to request new authorization
	if token == nil {
		state, err := MakeRequestID()
		if err != nil {
			return nil, err
		}
		requests.Set(state, msg)
		authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
		// strip protocol to skip unfurl prompt
		authURL = strings.TrimPrefix(authURL, "https://")
		_, err = kbc.SendMessageByTlfName(msg.Sender.Username, authMessageTemplate, authURL)
		return nil, err
	}

	return config.Client(context.Background(), token), nil
}
