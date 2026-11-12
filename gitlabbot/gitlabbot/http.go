package gitlabbot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/keybase/managed-bots/base/git"
	gitlab "gitlab.com/gitlab-org/api/client-go"

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

func NewHTTPSrv(
	stats *base.StatsRegistry,
	kbc *kbchat.API,
	debugConfig *base.ChatDebugOutputConfig,
	db *DB,
	handler *Handler,
	secret string,
) *HTTPSrv {
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

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, _ *http.Request) {
	if _, err := fmt.Fprintf(w, "beep boop! :)"); err != nil {
		h.Debug("handleHealthCheck: failed to write response: %s", err)
	}
}

// webhookBodyLimit caps a GitLab webhook body. Push events carry every commit
// in the push, so this is much larger than the other bots' limits.
const webhookBodyLimit = 25 << 20

// webhookProjectPath returns the lowercased project path of a GitLab event.
// Every supported event type keeps it at project.path_with_namespace, so it can
// be read before the full event is parsed and used to select the signing keys.
func webhookProjectPath(payload []byte) (string, error) {
	var evt struct {
		Project struct {
			PathWithNamespace string `json:"path_with_namespace"`
		} `json:"project"`
	}
	if err := json.Unmarshal(payload, &evt); err != nil {
		return "", err
	}
	return strings.ToLower(evt.Project.PathWithNamespace), nil
}

func (h *HTTPSrv) handleWebhook(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if cerr := r.Body.Close(); cerr != nil {
			h.Errorf("handleWebhook: failed to close request body: %s", cerr)
		}
	}()
	base.LimitRequestBody(w, r, webhookBodyLimit)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "webhook payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		h.Errorf("Error reading payload: %s", err)
		http.Error(w, "unable to read webhook payload", http.StatusBadRequest)
		return
	}

	// Authenticate the raw body before parsing or formatting it. Only the
	// project path is read here, to find the subscriptions whose keys apply.
	repo, err := webhookProjectPath(payload)
	if err != nil || repo == "" {
		h.Errorf("could not read webhook project: %v", err)
		http.Error(w, "invalid webhook payload", http.StatusBadRequest)
		return
	}
	webhookID := r.Header.Get("webhook-id")
	timestamp := r.Header.Get("webhook-timestamp")
	signatures := r.Header.Get("webhook-signature")
	legacyToken := r.Header.Get("X-Gitlab-Token")
	// WithoutCancel: GitLab sends the webhook and may close the connection
	// immediately; DB reads and Keybase message sends must complete regardless.
	ctx := context.WithoutCancel(r.Context())

	convs, err := h.db.GetSubscribedConvs(ctx, repo)
	if err != nil {
		h.Errorf("Error getting subscriptions for repo: %s", err)
		http.Error(w, "unable to process webhook", http.StatusInternalServerError)
		return
	}

	type authenticatedConv struct {
		conv SubscribedConv
		// completeReauth is set when a signature replaces a legacy token on a
		// hook that needs reauthorization; it is recorded once the event is valid.
		completeReauth bool
	}
	var authenticated []authenticatedConv
	now := time.Now()
	for _, conv := range convs {
		authMethod, err := authenticateWebhook(
			webhookSigningKey(repo, conv.ConvID, h.secret),
			legacyWebhookSecretToken(repo, conv.ConvID, h.secret),
			legacyToken,
			webhookID,
			timestamp,
			signatures,
			payload,
			now,
		)
		if err != nil {
			h.Debug("webhook authentication failed for conversation %s: %s", conv.ConvID, err)
			continue
		}
		if !webhookAuthenticationAllowed(authMethod, conv.ReauthorizationNeeded) {
			h.Debug("webhook authentication method is not allowed for conversation %s", conv.ConvID)
			continue
		}
		authenticated = append(authenticated, authenticatedConv{
			conv:           conv,
			completeReauth: conv.ReauthorizationNeeded && authMethod == webhookAuthenticationSignature,
		})
	}
	if len(authenticated) == 0 {
		http.Error(w, "invalid webhook authentication", http.StatusUnauthorized)
		return
	}

	event, err := gitlab.ParseWebhook(gitlab.WebhookEventType(r), payload)
	if err != nil {
		h.Errorf("could not parse webhook: type:%v %s\n", gitlab.WebhookEventType(r), err)
		http.Error(w, "invalid webhook payload", http.StatusBadRequest)
		return
	}

	var message string
	switch event := event.(type) {
	case *gitlab.IssueEvent:
		message = git.FormatIssueMsg(
			event.ObjectAttributes.Action,
			event.User.Username,
			event.Project.Name,
			int(event.ObjectAttributes.IID),
			event.ObjectAttributes.Title,
			event.ObjectAttributes.URL,
		)
	case *gitlab.MergeEvent:
		message = git.FormatPullRequestMsg(
			git.GITLAB,
			event.ObjectAttributes.Action,
			event.User.Username,
			event.Project.PathWithNamespace,
			int(event.ObjectAttributes.IID),
			event.ObjectAttributes.Title,
			event.ObjectAttributes.URL,
			event.ObjectAttributes.TargetBranch,
		)
	case *gitlab.PushEvent:
		if len(event.Commits) == 0 {
			break
		}
		branch := git.RefToName(event.Ref)
		commitMsgs := getCommitMessages(event)
		lastCommitDiffURL := event.Commits[len(event.Commits)-1].URL

		message = git.FormatPushMsg(
			event.UserUsername,
			event.Project.Name,
			branch,
			len(event.Commits),
			commitMsgs,
			lastCommitDiffURL,
		)
	case *gitlab.PipelineEvent:
		message = formatPipelineMsg(event, event.User.Username)
	}

	if message == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	processingFailed := false
	for _, a := range authenticated {
		if a.completeReauth {
			if err := h.db.CompleteSubscriptionReauthorization(ctx, a.conv.ConvID, repo); err != nil {
				h.Errorf("Error completing webhook reauthorization for conversation %s: %s", a.conv.ConvID, err)
				processingFailed = true
				continue
			}
		}
		h.ChatEcho(a.conv.ConvID, "%s", message)
	}

	if processingFailed {
		http.Error(w, "unable to process webhook", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
