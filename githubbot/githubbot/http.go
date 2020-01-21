package githubbot

import (
	"context"
	"fmt"
	"github.com/keybase/managed-bots/base/git"
	"io/ioutil"
	"net/http"

	"github.com/google/go-github/v28/github"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type HTTPSrv struct {
	*base.OAuthHTTPSrv

	config  *oauth2.Config
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
	h.OAuthHTTPSrv = base.NewOAuthHTTPSrv(kbc, config, requests, h.db, h.handler.HandleAuth,
		"githubbot", base.Images["logo"], "/githubbot")
	http.HandleFunc("/githubbot", h.handleHealthCheck)
	http.HandleFunc("/githubbot/webhook", h.handleWebhook)
	return h
}

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "beep boop! :)")
}

func (h *HTTPSrv) handleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.Debug("Error reading payload: %s\n", err)
		return
	}
	defer r.Body.Close()

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		h.Debug("could not parse webhook: %s\n", err)
		return
	}

	type genericPayload interface {
		GetRepo() *github.Repository
	}
	var repo string
	switch event := event.(type) {
	case *github.PushEvent:
		// PushEvent.GetRepo() returns *github.PushEventRepository instead of *github.Repository, so we handle it separately
		repo = event.GetRepo().GetFullName()
	default:
		if evt, ok := event.(genericPayload); ok {
			repo = evt.GetRepo().GetFullName()
		} else {
			h.Debug("could not get repo from webhook, webhook type: %s\n", github.WebHookType(r))
			return
		}
	}

	if repo != "" {
		signature := r.Header.Get("X-Hub-Signature")
		convs, err := h.db.GetConvIDsFromRepo(repo)
		if err != nil {
			h.Debug("Error getting subscriptions for repo: %s", err)
			return
		}

		for _, convID := range convs {
			if err = github.ValidateSignature(signature, payload, []byte(base.MakeSecret(repo, convID, h.secret))); err != nil {
				// if there's an error validating the signature for a conversation, don't send the message to that convo
				h.Debug("Error validating payload signature for conversation %s: %s", convID, err)
				continue
			}
			token, err := h.db.GetTokenFromConvID(convID)
			if err != nil {
				h.Debug("could not get token for convID: %s\n", err)
				return
			}
			client := github.NewClient(h.config.Client(context.TODO(), token))
			message, branch := h.formatMessage(event, repo, client)
			if message != "" {
				subscriptionExists, err := h.db.GetSubscriptionExists(convID, repo, branch)
				if err != nil {
					h.Debug("could not get subscription: %s\n", err)
					return
				}

				if subscriptionExists {
					_, err = h.kbc.SendMessageByConvID(convID, message)
					if err != nil {
						h.Debug("Error sending message: %s", err)
						return
					}
				}
			}
		}
	}
}

func (h *HTTPSrv) formatMessage(event interface{}, repo string, client *github.Client) (message string, branch string) {
	branch, err := getDefaultBranch(repo, client)
	if err != nil {
		h.Debug("formatMessage: error getting default branch: %s", err)
		return "", ""
	}
	switch event := event.(type) {
	case *github.IssuesEvent:
		author := getPossibleKBUser(h.kbc, h.db, h.DebugOutput, event.GetSender().GetLogin())
		message = git.FormatIssueMsg(
			*event.Action,
			author.String(),
			event.GetRepo().GetName(),
			event.GetIssue().GetNumber(),
			event.GetIssue().GetTitle(),
			event.GetIssue().GetHTMLURL(),
			)
	case *github.PullRequestEvent:
		var author username
		if event.GetPullRequest().GetMerged() {
			author = getPossibleKBUser(h.kbc, h.db, h.DebugOutput, event.GetPullRequest().GetMergedBy().GetLogin())
		} else {
			author = getPossibleKBUser(h.kbc, h.db, h.DebugOutput, event.GetSender().GetLogin())
		}

		action := *event.Action
		if event.GetPullRequest().GetMerged() {
			action = "merged"
		}

		message = git.FormatPullRequestMsg(
			git.GITHUB,
			action,
			author.String(),
			event.GetRepo().GetName(),
			event.GetNumber(),
			event.GetPullRequest().GetTitle(),
			event.GetPullRequest().GetHTMLURL(),
			event.GetRepo().GetName(),
			)
	case *github.PushEvent:
		if len(event.Commits) == 0 {
			break
		}

		branch = git.RefToName(event.GetRef())
		commitMsgs := getCommitMessages(event)

		message = git.FormatPushMsg(
			event.GetSender().GetLogin(),
			event.GetRepo().GetName(),
			branch,
			len(event.Commits),
			commitMsgs,
			event.GetCompare())


	case *github.CheckRunEvent:
		author := getPossibleKBUser(h.kbc, h.db, h.DebugOutput, event.GetSender().GetLogin())
		if len(event.GetCheckRun().PullRequests) == 0 {
			// this is a branch test, not associated with a PR
			branch = event.GetCheckRun().GetCheckSuite().GetHeadBranch()
		}
		// if we're parsing a pr, use the default branch
		message = formatCheckRunMessage(event, author.String())
	case *github.StatusEvent:
		author := getPossibleKBUser(h.kbc, h.db, h.DebugOutput, event.GetSender().GetLogin())
		pullRequests, _, err := client.PullRequests.ListPullRequestsWithCommit(
			context.TODO(),
			event.GetRepo().GetOwner().GetLogin(),
			event.GetRepo().GetName(),
			event.GetSHA(),
			&github.PullRequestListOptions{
				State: "open",
			},
		)
		if err != nil {
			h.Debug("error getting pull requests from commit: %s", err)
		}
		if len(pullRequests) == 0 && len(event.Branches) >= 1 {
			// this is a branch test, not associated with a PR
			branch = event.Branches[0].GetName()
		}
		message = formatStatusMessage(event, pullRequests, author.String())

	}
	return message, branch
}
