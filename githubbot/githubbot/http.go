package githubbot

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/google/go-github/v28/github"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
)

type HTTPSrv struct {
	kbc *kbchat.API
}

func NewHTTPSrv(kbc *kbchat.API) *HTTPSrv {
	return &HTTPSrv{
		kbc: kbc,
	}
}

func (h *HTTPSrv) debug(msg string, args ...interface{}) {
	fmt.Printf("Handler: "+msg+"\n", args...)
}

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "beep boop! :)")
}

func (h *HTTPSrv) handleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.debug("Error reading payload: %s", err)
		return
	}
	defer r.Body.Close()

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		h.debug("could not parse webhook: =%s\n", err)
		return
	}

	switch event := event.(type) {
	case *github.IssuesEvent:
		h.kbc.SendMessageByConvID(convID, formatIssueMsg(event))
	case *github.PullRequestEvent:
		h.kbc.SendMessageByConvID(convID, formatPRMsg(event))
		break
	case *github.PushEvent:
		if len(event.Commits) == 0 {
			break
		}
		h.kbc.SendMessageByConvID(convID, formatPushMsg(event))
		break
	default:
		break
	}

}

func (h *HTTPSrv) Listen() error {
	http.HandleFunc("/githubbot", h.handleHealthCheck)
	http.HandleFunc("/githubbot/webhook", h.handleWebhook)
	return http.ListenAndServe(":8081", nil) // TODO: make this configurable via opts?
}
