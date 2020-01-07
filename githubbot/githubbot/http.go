package githubbot

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/google/go-github/v28/github"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type HTTPSrv struct {
	*base.HTTPSrv

	kbc      *kbchat.API
	db       *DB
	handler  *Handler
	requests *base.OAuthRequests
	config   *oauth2.Config
	secret   string
}

func NewHTTPSrv(kbc *kbchat.API, db *DB, handler *Handler, requests *base.OAuthRequests, config *oauth2.Config, secret string) *HTTPSrv {
	h := &HTTPSrv{
		kbc:      kbc,
		db:       db,
		handler:  handler,
		requests: requests,
		config:   config,
		secret:   secret,
	}
	h.HTTPSrv = base.NewHTTPSrv(kbc)
	http.HandleFunc("/githubbot", h.handleHealthCheck)
	http.HandleFunc("/githubbot/webhook", h.handleWebhook)
	http.HandleFunc("/githubbot/oauth", h.handleOauth)
	return h
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
		h.Debug("could not parse webhook: %s\n", err)
		return
	}

	var message string
	var repo string
	branch := "master"
	switch event := event.(type) {
	case *github.IssuesEvent:
		author, err := getPossibleKBUser(h.kbc, event.GetSender().GetLogin())
		if err != nil {
			h.Debug("error getting keybase user: %s", err)
			return
		}
		message = formatIssueMsg(event, author)
		repo = event.GetRepo().GetFullName()
		branch, err = getDefaultBranch(repo, github.NewClient(nil))
		if err != nil {
			h.Debug("error getting default branch: %s", err)
			return
		}
	case *github.PullRequestEvent:
		author, err := getPossibleKBUser(h.kbc, event.GetSender().GetLogin())
		if err != nil {
			h.Debug("error getting keybase user: %s", err)
			return
		}
		message = formatPRMsg(event, author)
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
		author, err := getPossibleKBUser(h.kbc, event.GetSender().GetLogin())
		if err != nil {
			h.Debug("error getting keybase user: %s", err)
			return
		}
		message = formatPushMsg(event, author)
		repo = event.GetRepo().GetFullName()
		branch = refToName(event.GetRef())
	case *github.CheckSuiteEvent:
		author, err := getPossibleKBUser(h.kbc, event.GetSender().GetLogin())
		if err != nil {
			h.Debug("error getting keybase user: %s", err)
			return
		}
		repo = event.GetRepo().GetFullName()
		if len(event.GetCheckSuite().PullRequests) == 0 {
			// this is a branch test, not associated with a PR
			branch = event.GetCheckSuite().GetHeadBranch()
		} else {
			branch, err = getDefaultBranch(repo, github.NewClient(nil))
		}
		message = formatCheckSuiteMsg(event, author)
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

func (h *HTTPSrv) handleOauth(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			h.Debug("oauthHandler: %v", err)
			if _, err := w.Write(base.AsHTML("githubbot", "error", "Unable to complete request, please try again!", "")); err != nil {
				h.Debug("oauthHandler: unable to write: %v", err)
			}
		}
	}()

	if r.URL == nil {
		err = fmt.Errorf("r.URL == nil")
		return
	}

	query := r.URL.Query()
	state := query.Get("state")

	h.requests.Lock()
	originatingMsg, ok := h.requests.Requests[state]
	delete(h.requests.Requests, state)
	h.requests.Unlock()
	if !ok {
		err = fmt.Errorf("state %q not found %v", state, h.requests)
		return
	}

	code := query.Get("code")
	token, err := h.config.Exchange(context.TODO(), code)
	if err != nil {
		return
	}

	if err = h.db.PutToken(base.IdentifierFromMsg(originatingMsg), token); err != nil {
		return
	}

	if err = h.handler.HandleCommand(originatingMsg); err != nil {
		return
	}

	if _, err := w.Write(base.AsHTML("githubbot", "success", "Success! You can now close this page and return to the Keybase app.", "")); err != nil {
		h.Debug("oauthHandler: unable to write: %v", err)
	}
}
