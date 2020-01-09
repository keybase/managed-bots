package githubbot

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/google/go-github/v28/github"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type HTTPSrv struct {
	*base.OAuthHTTPSrv

	kbc     *kbchat.API
	db      *DB
	handler *Handler
	secret  string
}

func NewHTTPSrv(kbc *kbchat.API, db *DB, handler *Handler, requests *base.OAuthRequests, config *oauth2.Config, secret string) *HTTPSrv {
	h := &HTTPSrv{
		kbc:     kbc,
		db:      db,
		handler: handler,
		secret:  secret,
	}
	h.OAuthHTTPSrv = base.NewOAuthHTTPSrv(kbc, config, requests, h.db.PutToken, h.handler.HandleCommand, "githubbot", "/githubbot/logo.png", "/githubbot")
	http.HandleFunc("/githubbot", h.handleHealthCheck)
	http.HandleFunc("/githubbot/logo.png", h.handleLogo)
	http.HandleFunc("/githubbot/webhook", h.handleWebhook)
	return h
}

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "beep boop! :)")
}

func (h *HTTPSrv) handleLogo(w http.ResponseWriter, r *http.Request) {
	path := fmt.Sprintf("/keybase/private/%s/static/logo.png", h.kbc.GetUsername())
	id := h.kbc.Command("fs", "read", path)
	logo, err := id.Output()
	if err != nil {
		h.Debug("Error reading logo: %s", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if _, err := w.Write(logo); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	return
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
		h.Debug("could not parse webhook: %s\n", err)
		return
	}

	var message string
	var repo string
	branch := "master"
	switch event := event.(type) {
	case *github.IssuesEvent:
		author := getPossibleKBUser(h.kbc, h.DebugOutput, event.GetSender().GetLogin())
		message = formatIssueMsg(event, author.String())
		repo = event.GetRepo().GetFullName()
		branch, err = getDefaultBranch(repo, github.NewClient(nil))
		if err != nil {
			h.Debug("error getting default branch: %s", err)
			return
		}
	case *github.PullRequestEvent:
		var author username
		if event.GetPullRequest().GetMerged() {
			author = getPossibleKBUser(h.kbc, h.DebugOutput, event.GetPullRequest().GetMergedBy().GetLogin())
		} else {
			author = getPossibleKBUser(h.kbc, h.DebugOutput, event.GetSender().GetLogin())
		}
		message = formatPRMsg(event, author.String())
		repo = event.GetRepo().GetFullName()

		branch, err = getDefaultBranch(repo, github.NewClient(nil))
		if err != nil {
			h.Debug("error getting default branch: %s", err)
			return
		}
	case *github.PushEvent:
		if len(event.Commits) == 0 {
			break
		}
		author := getPossibleKBUser(h.kbc, h.DebugOutput, event.GetSender().GetLogin())
		message = formatPushMsg(event, author.String())
		repo = event.GetRepo().GetFullName()
		branch = refToName(event.GetRef())
	case *github.CheckSuiteEvent:
		author := getPossibleKBUser(h.kbc, h.DebugOutput, event.GetSender().GetLogin())
		repo = event.GetRepo().GetFullName()
		if len(event.GetCheckSuite().PullRequests) == 0 {
			// this is a branch test, not associated with a PR
			branch = event.GetCheckSuite().GetHeadBranch()
		} else {
			branch, err = getDefaultBranch(repo, github.NewClient(nil))
		}
		message = formatCheckSuiteMsg(event, author.String())
		if err != nil {
			h.Debug("error getting default branch: %s", err)
			return
		}
	}

	if message != "" && repo != "" {
		signature := r.Header.Get("X-Hub-Signature")

		convs, err := h.db.GetSubscribedConvs(repo, branch)
		if err != nil {
			h.Debug("Error getting subscriptions for repo: %s", err)
			return
		}

		for _, convID := range convs {
			if err = github.ValidateSignature(signature, payload, []byte(makeSecret(repo, convID, h.secret))); err != nil {
				// if there's an error validating the signature for a conversation, don't send the message to that convo
				h.Debug("Error validating payload signature for conversation %s: %s", convID, err)
				continue
			}
			_, err = h.kbc.SendMessageByConvID(string(convID), message)
			if err != nil {
				h.Debug("Error sending message: %s", err)
				return
			}
		}
	}
}
