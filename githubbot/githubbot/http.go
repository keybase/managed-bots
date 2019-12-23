package githubbot

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/google/go-github/v28/github"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.DebugOutput

	kbc    *kbchat.API
	db     *DB
	secret string
}

func NewHTTPSrv(kbc *kbchat.API, db *DB, secret string) *HTTPSrv {
	return &HTTPSrv{
		DebugOutput: base.NewDebugOutput("HTTPSrv", kbc),
		kbc:         kbc,
		db:          db,
		secret:      secret,
	}
}

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "beep boop! :)")
}

func (h *HTTPSrv) handleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.Debug("Error reading payload: %s", err)
		return
	}
	defer r.Body.Close()

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		h.Debug("could not parse webhook: =%s\n", err)
		return
	}

	var message string
	var repo string
	branch := "master"
	switch event := event.(type) {
	case *github.IssuesEvent:
		message = formatIssueMsg(event)
		repo = event.GetRepo().GetFullName()
		branch, err = getDefaultBranch(repo)
		if err != nil {
			h.Debug("error getting default branch: %s", err)
			return
		}
	case *github.PullRequestEvent:
		message = formatPRMsg(event)
		repo = event.GetRepo().GetFullName()
		branch, err = getDefaultBranch(repo)
		if err != nil {
			h.Debug("error getting default branch: %s", err)
			return
		}
	case *github.PushEvent:
		if len(event.Commits) == 0 {
			break
		}

		message = formatPushMsg(event)
		repo = event.GetRepo().GetFullName()
		branch = refToName(event.GetRef())
	case *github.CheckSuiteEvent:
		if len(event.GetCheckSuite().PullRequests) == 0 {
			// this is a branch test, not associated with a PR
			branch = event.GetCheckSuite().GetHeadBranch()
		} else {
			branch, err = getDefaultBranch(repo)
		}
		message = formatCheckSuiteMsg(event)
		repo = event.GetRepo().GetFullName()
		if err != nil {
			h.Debug("error getting default branch: %s", err)
			return
		}
	}

	if message != "" && repo != "" {
		signature := r.Header.Get("X-Hub-Signature")
		if err = github.ValidateSignature(signature, payload, []byte(makeSecret(repo, h.secret))); err != nil {
			h.Debug("Error validating payload signature: %s", err)
			return
		}

		convs, err := h.db.GetSubscribedConvs(repo, branch)
		if err != nil {
			h.Debug("Error getting subscriptions for repo: %s", err)
			return
		}

		for _, convID := range convs {
			_, err = h.kbc.SendMessageByConvID(convID, message)
			if err != nil {
				h.Debug("Error sending message: %s", err)
				return
			}
		}
	}
}

func (h *HTTPSrv) Listen() error {
	http.HandleFunc("/githubbot", h.handleHealthCheck)
	http.HandleFunc("/githubbot/webhook", h.handleWebhook)
	return http.ListenAndServe(":8080", nil)
}
