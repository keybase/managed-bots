package gitlabbot

import (
	"fmt"
	"github.com/xanzy/go-gitlab"
	"io/ioutil"
	"net/http"
	"strings"

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
	h.OAuthHTTPSrv = base.NewOAuthHTTPSrv(kbc, config, requests, h.db, h.handler.HandleAuth,
		"gitlabbot", base.Images["logo"], "/gitlabbot")
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
		h.Debug("Error reading payload: %s", err)
		return
	}
	defer r.Body.Close()

	event, err := gitlab.ParseWebhook(gitlab.WebhookEventType(r), payload)
	if err != nil {
		h.Debug("could not parse webhook: %s\n", err)
		return
	}

	var message string
	var repo string
	branch := "master"
	switch event := event.(type) {
	case *gitlab.IssueEvent:
		author := getPossibleKBUser(h.kbc, h.db, h.DebugOutput, event.User.Username)
		message = formatIssueMsg(event, author.String())
		repo = strings.ToLower(event.Project.PathWithNamespace)
		//branch, err = getDefaultBranch(repo, github.NewClient(nil))
		//if err != nil {
		//	h.Debug("error getting default branch: %s", err)
		//	return
		//}
	case *gitlab.MergeEvent:
		var author username
		if event.ObjectAttributes.State == "merged" {
			author = getPossibleKBUser(h.kbc, h.db, h.DebugOutput, event.User.Username)
		} else {
			author = getPossibleKBUser(h.kbc, h.db, h.DebugOutput, event.User.Username)
		}
		message = formatMRMsg(event, author.String())
		repo = strings.ToLower(event.Project.PathWithNamespace)

		//branch, err = getDefaultBranch(repo, github.NewClient(nil))
		//if err != nil {
		//	h.Debug("error getting default branch: %s", err)
		//	return
		//}
	case *gitlab.PushEvent:
		if len(event.Commits) == 0 {
			break
		}
		message = formatPushMsg(event, event.UserUsername)
		repo = strings.ToLower(event.Project.PathWithNamespace)
		branch = refToName(event.Ref)
	case *gitlab.PipelineEvent:
		author := getPossibleKBUser(h.kbc, h.db, h.DebugOutput, event.User.Username)
		repo = strings.ToLower(event.Project.PathWithNamespace)
		//if len(event.GetCheckSuite().PullRequests) == 0 {
		//	// this is a branch test, not associated with a PR
		//	branch = event.GetCheckSuite().GetHeadBranch()
		//} else {
		//	branch, err = getDefaultBranch(repo, github.NewClient(nil))
		//	if err != nil {
		//		h.Debug("error getting default branch: %s", err)
		//		return
		//	}
		//}
		message = formatPipelineMsg(event, author.String())
	}

	if message != "" && repo != "" {
		signature := r.Header.Get("X-Gitlab-Token")

		convs, err := h.db.GetSubscribedConvs(repo, branch)
		if err != nil {
			h.Debug("Error getting subscriptions for repo: %s", err)
			return
		}

		for _, convID := range convs {
			var secretToken = makeSecret(repo, convID, h.secret)
			if signature != secretToken {
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
