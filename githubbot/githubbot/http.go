package githubbot

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"

	"github.com/keybase/managed-bots/base/git"

	"github.com/bradleyfalzon/ghinstallation"

	"github.com/google/go-github/v31/github"
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

func NewHTTPSrv(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig, db *DB, handler *Handler,
	oauthConfig *oauth2.Config, atr *ghinstallation.AppsTransport, secret string) *HTTPSrv {
	h := &HTTPSrv{
		kbc:     kbc,
		db:      db,
		handler: handler,
		atr:     atr,
		secret:  secret,
	}
	h.OAuthHTTPSrv = base.NewOAuthHTTPSrv(stats, kbc, debugConfig, oauthConfig, h.db, h.handler.HandleAuth,
		"githubbot", base.Images["logo"], "/githubbot")
	http.HandleFunc("/githubbot", h.handleHealthCheck)
	http.HandleFunc("/githubbot/webhook", h.handleWebhook)
	return h
}

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "beep boop! :)")
}

func (h *HTTPSrv) handleWebhook(_ http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, []byte(h.secret))
	if err != nil {
		h.Debug("Error validating payload (%s): %v\n", r.Header.Get("X-GitHub-Delivery"), err)
		h.Stats.Count("webhook - invalid payload")
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		h.Debug("could not parse webhook: type:%s %s\n", github.WebHookType(r), err)
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
			h.Debug("could not get information from webhook, webhook type: %s\n", github.WebHookType(r))
			return
		}
	}

	itr := ghinstallation.NewFromAppsTransport(h.atr, installationID)
	client := github.NewClient(&http.Client{Transport: itr})

	if repo == "" {
		return
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
			h.Stats.Count("webhook - disabled feature")
			continue
		}

		message, branch := h.formatMessage(convID, event, repo, client)
		if message == "" {
			// if we don't have a message to send, bail
			continue
		}

		if branch != "" {
			// if the event has a branch associated with it, check if we're subscribed to that branch
			subscriptionExists, err := h.db.GetSubscriptionForBranchExists(convID, repo, branch)
			if err != nil {
				h.Errorf("could not get subscription: %s\n", err)
				return
			}

			if !subscriptionExists {
				continue
			}
		}

		h.Stats.Count("webhook - success")
		h.ChatEcho(convID, "%s", message)
	}
}

func (h *HTTPSrv) formatMessage(convID chat1.ConvIDStr, event interface{}, repo string, client *github.Client) (message string, branch string) {
	parsedRepo := strings.Split(repo, "/")
	if len(parsedRepo) != 2 {
		h.Debug("invalid repo: %s", repo)
		return
	}
	switch event := event.(type) {
	case *github.IssuesEvent:
		author := getPossibleKBUser(h.kbc, h.db, h.DebugOutput, event.GetSender().GetLogin(), convID)
		return git.FormatIssueMsg(
			*event.Action,
			author.String(),
			event.GetRepo().GetName(),
			event.GetIssue().GetNumber(),
			event.GetIssue().GetTitle(),
			event.GetIssue().GetHTMLURL(),
		), ""
	case *github.ReleaseEvent:
		author := getPossibleKBUser(h.kbc, h.db, h.DebugOutput, event.GetSender().GetLogin(), convID)
		return git.FormatReleaseMsg(
			*event.Action,
			author.String(),
			event.GetRepo().GetName(),
			event.GetRelease().GetTagName(),
			event.GetRelease().GetName(),
			event.GetRelease().GetURL(),
			event.GetRelease().GetBody(),
		), ""
	case *github.PullRequestEvent:
		var author username
		if event.GetPullRequest().GetMerged() {
			author = getPossibleKBUser(h.kbc, h.db, h.DebugOutput, event.GetPullRequest().GetMergedBy().GetLogin(), convID)
		} else {
			author = getPossibleKBUser(h.kbc, h.db, h.DebugOutput, event.GetPullRequest().GetUser().GetLogin(), convID)
		}

		action := *event.Action
		if event.GetPullRequest().GetMerged() {
			action = "merged"
		}

		return git.FormatPullRequestMsg(
			git.GITHUB,
			action,
			author.String(),
			event.GetRepo().GetName(),
			event.GetNumber(),
			event.GetPullRequest().GetTitle(),
			event.GetPullRequest().GetHTMLURL(),
			event.GetRepo().GetName(),
		), ""
	case *github.PushEvent:
		if len(event.Commits) == 0 {
			break
		}

		branch = git.RefToName(event.GetRef())
		commitMsgs := getCommitMessages(event)

		return git.FormatPushMsg(
			event.GetSender().GetLogin(),
			event.GetRepo().GetName(),
			branch,
			len(event.Commits),
			commitMsgs,
			event.GetCompare()), branch

	case *github.CheckRunEvent:
		var author username

		// this is a branch test, not associated with a PR
		var runPR *github.PullRequest
		// the repo object returned by the GetCheckRun call is very sparse, so we really only can check against the api url
		repoAPIUrl := fmt.Sprintf("https://api.github.com/repos/%s", repo)
		for _, pr := range event.GetCheckRun().PullRequests {
			if pr.GetBase().GetRepo().GetURL() == repoAPIUrl {
				runPR = pr
				break
			}
		}
		if runPR == nil {
			// this is a branch test, not associated with a PR
			branch = event.GetCheckRun().GetCheckSuite().GetHeadBranch()
			// don't provide an author since it's not a PR
			return formatCheckRunMessage(event, ""), branch
		}

		// fetch the pull request object so we can get the right author
		pr, _, err := client.PullRequests.Get(context.TODO(), parsedRepo[0], parsedRepo[1], runPR.GetNumber())
		if err != nil {
			if !strings.Contains(err.Error(), "401 Bad credentials") {
				h.Errorf("Error getting pull request object: %s", err)
			}
			return formatCheckRunMessage(event, ""), branch
		}
		author = getPossibleKBUser(h.kbc, h.db, h.DebugOutput, pr.GetUser().GetLogin(), convID)
		return formatCheckRunMessage(event, author.String()), branch

	case *github.StatusEvent:
		var author username
		pullRequests, _, err := client.PullRequests.ListPullRequestsWithCommit(
			context.TODO(),
			event.GetRepo().GetOwner().GetLogin(),
			event.GetRepo().GetName(),
			event.GetSHA(),
			&github.PullRequestListOptions{
				State:     "open",
				Sort:      "updated",
				Direction: "desc",
			},
		)
		if err != nil && !strings.Contains(err.Error(), "401 Bad credentials") {
			h.Errorf("error getting pull requests from commit: %s", err)
		}

		// look for PR where the base is the provided repo
		var runPR *github.PullRequest
		for _, pr := range pullRequests {
			if pr.GetBase().GetRepo().GetFullName() == repo {
				runPR = pr
				break
			}
		}

		if runPR != nil {
			author = getPossibleKBUser(h.kbc, h.db, h.DebugOutput, runPR.GetUser().GetLogin(), convID)
		} else if len(event.Branches) >= 1 {
			// this is a branch test, not associated with a PR
			branch = event.Branches[0].GetName()
			author = getPossibleKBUser(h.kbc, h.db, h.DebugOutput, event.GetCommit().GetAuthor().GetLogin(), convID)
		} else {
			h.Debug("status event had no pull requests or branches")
			return "", ""
		}

		return formatStatusMessage(event, pullRequests, author.String()), branch
	}
	return "", ""
}
