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
	HTTPSrv
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
	o.DebugOutput = NewDebugOutput("HTTPSrv", kbc)
	o.srv = &http.Server{Addr: ":8080"}
	oauthHandler := o.makeOauthHandler()
	http.HandleFunc(urlPrefix+"/oauth", oauthHandler)
	return o
}

func (o *OAuthHTTPSrv) makeOauthHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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

		o.requests.Lock()
		originatingMsg, ok := o.requests.Map[state]
		delete(o.requests.Map, state)
		o.requests.Unlock()
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
}

type OAuthRequests struct {
	sync.Mutex

	Map map[string]chat1.MsgSummary
}

func NewOAuthRequests() *OAuthRequests {
	return &OAuthRequests{
		Map: make(map[string]chat1.MsgSummary),
	}
}
