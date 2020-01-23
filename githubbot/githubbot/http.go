package githubbot

import (
	"context"
	"fmt"
	"net/http"

	"github.com/keybase/managed-bots/base/git"

	"github.com/bradleyfalzon/ghinstallation"

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
	atr     *ghinstallation.AppsTransport
	secret  string
}

func NewHTTPSrv(kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig, db *DB, handler *Handler,
	requests *base.OAuthRequests, oauthConfig *oauth2.Config, atr *ghinstallation.AppsTransport, secret string) *HTTPSrv {
	h := &HTTPSrv{
		kbc:     kbc,
		db:      db,
		handler: handler,
		atr:     atr,
		secret:  secret,
	}
	h.OAuthHTTPSrv = base.NewOAuthHTTPSrv(kbc, debugConfig, oauthConfig, requests, h.db, h.handler.HandleAuth,
		"githubbot", base.Images["logo"], "/githubbot")
	http.HandleFunc("/githubbot", h.handleHealthCheck)
	http.HandleFunc("/githubbot/webhook", h.handleWebhook)
	return h
}

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "beep boop! :)")
}

func (h *HTTPSrv) handleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, []byte(h.secret))
	if err != nil {
		h.Errorf("Error validating payload: %s\n", err)
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		h.Errorf("could not parse webhook: %s\n", err)
		return
	}

	type genericPayload interface {
		GetRepo() *github.Repository
		GetInstallation() *github.Installation
	}
	var repo string
	var installationID int64
	switch event := event.(type) {
	case *github.PushEvent:
		// PushEvent.GetRepo() returns *github.PushEventRepository instead of *github.Repository, so we handle it separately
		repo = event.GetRepo().GetFullName()
		installationID = event.GetInstallation().GetID()
	default:
		if evt, ok := event.(genericPayload); ok {
			repo = evt.GetRepo().GetFullName()
			installationID = evt.GetInstallation().GetID()
		} else {
			h.Errorf("could not get information from webhook, webhook type: %s\n", github.WebHookType(r))
			return
		}
	}

	itr := ghinstallation.NewFromAppsTransport(h.atr, installationID)
	client := github.NewClient(&http.Client{Transport: itr})

	if repo != "" {
		defaultBranch, err := getDefaultBranch(repo, client)
		if err != nil {
			h.Errorf("Error getting default branch: %s", err)
		}

		convs, err := h.db.GetConvIDsFromRepoInstallation(repo, installationID)
		if err != nil {
			h.Errorf("Error getting subscriptions for repo: %s", err)
			return
		}

		for _, convID := range convs {
			features, err := h.db.GetFeatures(convID, repo)
			if err != nil {
				h.Errorf("Error getting features for repo and convID: %s", err)
				return
			}

			if !shouldParseEvent(event, features) {
				// If a conversation is not subscribed to the feature an event is part of, bail
				continue
			}

			message, branch := h.formatMessage(event, repo, defaultBranch, client)
			if message == "" {
				// if we don't have a message to send, bail
				continue
			}

			if branch != defaultBranch {
				// if the event is not on the default branch, check if we're subscribed to that branch
				subscriptionExists, err := h.db.GetSubscriptionForBranchExists(convID, repo, branch)
				if err != nil {
					h.Errorf("could not get subscription: %s\n", err)
					return
				}

				if !subscriptionExists {
					continue
				}
			}

			_, err = h.kbc.SendMessageByConvID(convID, message)
			if err != nil {
				h.Debug("Error sending message: %s", err)
				return
			}

		}
	}
}

func (h *HTTPSrv) formatMessage(event interface{}, repo string, defaultBranch string, client *github.Client) (message string, branch string) {
	branch = defaultBranch
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
			h.Errorf("error getting pull requests from commit: %s", err)
		}
		if len(pullRequests) == 0 && len(event.Branches) >= 1 {
			// this is a branch test, not associated with a PR
			branch = event.Branches[0].GetName()
		}
		message = formatStatusMessage(event, pullRequests, author.String())

	}
	return message, branch
}
