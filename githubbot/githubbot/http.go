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
	db  *DB
}

func NewHTTPSrv(kbc *kbchat.API, db *DB) *HTTPSrv {
	return &HTTPSrv{
		kbc: kbc,
		db:  db,
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

	var message string
	var repo string
	switch event := event.(type) {
	case *github.IssuesEvent:
		message = formatIssueMsg(event)
		repo = event.GetRepo().GetFullName()
		break
	case *github.PullRequestEvent:
		message = formatPRMsg(event)
		repo = event.GetRepo().GetFullName()
		break
	case *github.PushEvent:
		if len(event.Commits) == 0 {
			break
		}
		// TODO: implement branch filtering
		message = formatPushMsg(event)
		repo = event.GetRepo().GetFullName()
		break
	default:
		break
	}

	if message != "" && repo != "" {
		convs, err := h.db.GetSubscribedConvs(repo)
		if err != nil {
			h.debug("Error getting subscriptions for repo: %s", err)
			return
		}

		for _, convID := range convs {
			h.kbc.SendMessageByConvID(convID, message)
		}
	}
}

func (h *HTTPSrv) Listen() error {
	http.HandleFunc("/githubbot", h.handleHealthCheck)
	http.HandleFunc("/githubbot/webhook", h.handleWebhook)
	return http.ListenAndServe(":8081", nil) // TODO: make this configurable via opts?
}
