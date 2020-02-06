package gitlabbot

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/keybase/managed-bots/base/git"
	"github.com/xanzy/go-gitlab"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.HTTPSrv

	kbc     *kbchat.API
	db      *DB
	handler *Handler
	secret  string
}

func NewHTTPSrv(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig,
	db *DB, handler *Handler, secret string) *HTTPSrv {
	h := &HTTPSrv{
		kbc:     kbc,
		db:      db,
		handler: handler,
		secret:  secret,
	}
	h.HTTPSrv = base.NewHTTPSrv(stats, debugConfig)
	http.HandleFunc("/gitlabbot", h.handleHealthCheck)
	http.HandleFunc("/gitlabbot/webhook", h.handleWebhook)
	return h
}

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "beep boop! :)")
}

func (h *HTTPSrv) handleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.Errorf("Error reading payload: %s", err)
		return
	}
	defer r.Body.Close()

	event, err := gitlab.ParseWebhook(gitlab.WebhookEventType(r), payload)
	if err != nil {
		h.Errorf("could not parse webhook: type:%v %s\n", gitlab.WebhookEventType(r), err)
		return
	}

	var message, repo string
	branch := "master"
	switch event := event.(type) {
	case *gitlab.IssueEvent:
		message = git.FormatIssueMsg(
			event.ObjectAttributes.Action,
			event.User.Username,
			event.Project.Name,
			event.ObjectAttributes.IID,
			event.ObjectAttributes.Title,
			event.ObjectAttributes.URL,
		)
		repo = event.Project.PathWithNamespace
		branch = event.Project.DefaultBranch
	case *gitlab.MergeEvent:
		message = git.FormatPullRequestMsg(
			git.GITLAB,
			event.ObjectAttributes.Action,
			event.User.Username,
			event.Project.PathWithNamespace,
			event.ObjectAttributes.IID,
			event.ObjectAttributes.Title,
			event.ObjectAttributes.URL,
			event.ObjectAttributes.TargetBranch,
		)
		repo = event.Project.PathWithNamespace
		branch = event.Project.DefaultBranch
	case *gitlab.PushEvent:
		if len(event.Commits) == 0 {
			break
		}
		branch = git.RefToName(event.Ref)
		commitMsgs := getCommitMessages(event)
		lastCommitDiffURL := event.Commits[len(event.Commits)-1].URL

		message = git.FormatPushMsg(
			event.UserUsername,
			event.Project.Name,
			branch,
			len(event.Commits),
			commitMsgs,
			lastCommitDiffURL)
		repo = event.Project.PathWithNamespace
	case *gitlab.PipelineEvent:
		repo = event.Project.PathWithNamespace
		if event.MergeRequest.IID == 0 {
			branch = event.ObjectAttributes.Ref
		} else {
			branch = event.Project.DefaultBranch
		}
		message = formatPipelineMsg(event, event.User.Username)
	}

	if message == "" || repo == "" {
		return
	}
	repo = strings.ToLower(repo)
	branch = strings.ToLower(branch)
	signature := r.Header.Get("X-Gitlab-Token")

	convs, err := h.db.GetSubscribedConvs(repo, branch)
	if err != nil {
		h.Errorf("Error getting subscriptions for repo: %s", err)
		return
	}

	h.notifyUnsubscribedConvs(repo, branch, signature)

	for _, convID := range convs {
		var secretToken = base.MakeSecret(repo, convID, h.secret)
		if signature != secretToken {
			h.Debug("Error validating payload signature for conversation %s: %s", convID, err)
			continue
		}
		h.ChatEcho(convID, message)
	}
}

func (h *HTTPSrv) notifyUnsubscribedConvs(repo string, branch string, signature string) {
	convsToNotify, err := h.db.GetNotifiedBranches(repo, branch)
	if err != nil {
		h.Errorf("Error getting notified branches for repo: %s", err)
		return
	}

	message := formatNotifyBranchMsg(repo, branch)

	for _, convID := range convsToNotify {
		var secretToken = base.MakeSecret(repo, convID, h.secret)
		if signature != secretToken {
			h.Debug("Error validating payload signature for conversation %s: %s", convID, err)
			continue
		}
		h.ChatEcho(convID, message)

		err = h.db.PutNotifiedBranchConv(convID, repo, branch)
		if err != nil {
			h.Errorf("Error putting notified branch for repo: %s", err)
			return
		}
	}
}
