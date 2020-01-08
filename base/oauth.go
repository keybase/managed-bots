package base

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"golang.org/x/oauth2"
)

type OAuthHTTPSrv struct {
	*HTTPSrv
	oauth       *oauth2.Config
	requests    *OAuthRequests
	putToken    func(string, *oauth2.Token) error
	callback    func(chat1.MsgSummary) error
	htmlTitle   string
	htmlLogoSrc string
}

func NewOAuthHTTPSrv(
	kbc *kbchat.API,
	oauth *oauth2.Config,
	requests *OAuthRequests,
	putToken func(string, *oauth2.Token) error,
	callback func(chat1.MsgSummary) error,
	htmlTitle string,
	htmlLogoSrc string,
	urlPrefix string,
) *OAuthHTTPSrv {
	o := &OAuthHTTPSrv{
		oauth:       oauth,
		requests:    requests,
		putToken:    putToken,
		callback:    callback,
		htmlTitle:   htmlTitle,
		htmlLogoSrc: htmlLogoSrc,
	}
	o.HTTPSrv = NewHTTPSrv(kbc)
	http.HandleFunc(urlPrefix+"/oauth", o.oauthHandler)
	return o
}

func (o *OAuthHTTPSrv) oauthHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			o.Debug("oauthHandler: %v", err)
			if _, err := w.Write(AsHTML(o.htmlTitle, "error", "Unable to complete request, please try again!", o.htmlLogoSrc)); err != nil {
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

	if err = o.putToken(IdentifierFromMsg(originatingMsg), token); err != nil {
		return
	}

	if err = o.callback(originatingMsg); err != nil {
		return
	}

	if _, err := w.Write(AsHTML(o.htmlTitle, "success", "Success! You can now close this page and return to the Keybase app.", o.htmlLogoSrc)); err != nil {
		o.Debug("oauthHandler: unable to write: %v", err)
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
