package gitlabbot

import (
	"fmt"
	"github.com/kballard/go-shellquote"
	"github.com/xanzy/go-gitlab"
	"net/http"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type Handler struct {
	*base.DebugOutput

	kbc        *kbchat.API
	db         *DB
	requests   *base.OAuthRequests
	config     *oauth2.Config
	httpPrefix string
	secret     string
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(kbc *kbchat.API, db *DB, requests *base.OAuthRequests, config *oauth2.Config, httpPrefix string, secret string) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", kbc),
		kbc:         kbc,
		db:          db,
		requests:    requests,
		config:      config,
		httpPrefix:  httpPrefix,
		secret:      secret,
	}
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "Hi! I can notify you whenever something happens on a GitLab repository. To get started, set up a repository by sending `!gitlab subscribe <username/repo>`"
	return base.HandleNewTeam(h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleAuth(msg chat1.MsgSummary, _ string) error {
	return h.HandleCommand(msg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		h.Debug("skipping non-text message")
		return nil
	}

	cmd := strings.ToLower(strings.TrimSpace(msg.Content.Text.Body))
	if !strings.HasPrefix(cmd, "!gitlab") {
		h.Debug("ignoring non-command message")
		return nil
	}

	identifier := base.IdentifierFromMsg(msg)
	tc, err := base.GetOAuthClient(identifier, msg, h.kbc, h.requests, h.config, h.db,
		base.GetOAuthOpts{
			AuthMessageTemplate: "Visit %s\n to authorize me to set up GitLab notifications.",
		})
	if err != nil || tc == nil {
		return err
	}

	token, err := h.db.GetToken(identifier)
	if err != nil {
		h.Debug("error getting token from db")
		return err
	}

	client := gitlab.NewOAuthClient(tc, token.AccessToken)

	switch {
	case strings.HasPrefix(cmd, "!gitlab subscribe"):
		return h.handleSubscribe(cmd, msg, true, client)
	case strings.HasPrefix(cmd, "!gitlab unsubscribe"):
		return h.handleSubscribe(cmd, msg, false, client)
	}
	return nil
}

func (h *Handler) handleSubscribe(cmd string, msg chat1.MsgSummary, create bool, client *gitlab.Client) (err error) {
	toks, err := shellquote.Split(cmd)
	if err != nil {
		return fmt.Errorf("error splitting command: %s", err)
	}
	args := toks[2:]
	if len(args) < 1 {
		return fmt.Errorf("bad args for subscribe: %v", args)
	}

	// Check if command is subscribing to a branch
	if len(toks) == 4 {
		return h.handleSubscribeToBranch(cmd, msg, create, client)
	}


	var message string
	defer func() {
		if message != "" {
			_, err = h.kbc.SendMessageByConvID(msg.ConvID, fmt.Sprintf(message, args[0]))
			if err != nil {
				err = fmt.Errorf("error sending message: %s", err)
			}
		}
	}()

	alreadyExists, err := h.db.GetSubscriptionForRepoExists(msg.ConvID, args[0])
	if err != nil {
		return fmt.Errorf("error checking subscription: %s", err)
	}

	parsedRepo := strings.Split(args[0], "/")
	if len(parsedRepo) != 2 {
		return fmt.Errorf("invalid repo: %s", args[0])
	}
	if create {
		if !alreadyExists {
			project, err := getProject(args[0], client)
			if err != nil {
				return fmt.Errorf("error getting gitlab project: %s", err)
			}

			var webhookURL = h.httpPrefix + "/gitlabbot/webhook"
			var eventsFlag = true
			var token = makeSecret(args[0], msg.ConvID, h.secret)
			var defaultBranch = project.DefaultBranch

			hook, res, err := client.Projects.AddProjectHook(args[0], &gitlab.AddProjectHookOptions{
				URL:                      &webhookURL,
				PushEvents:               &eventsFlag,
				IssuesEvents:             &eventsFlag,
				ConfidentialIssuesEvents: &eventsFlag,
				MergeRequestsEvents:      &eventsFlag,
				PipelineEvents:           &eventsFlag,
				EnableSSLVerification:    nil,
				Token:                    &token,
			})

			if err != nil {
				if res.StatusCode != http.StatusNotFound {
					return fmt.Errorf("error: %s", err)
				}
				message = "I couldn't subscribe to updates on %s, do you have the right permissions?"
				return nil
			}

			err = h.db.CreateSubscription(msg.ConvID, args[0], defaultBranch, int64(hook.ID))
			if err != nil {
				return fmt.Errorf("error creating subscription: %s", err)
			}
			message = "Okay, you'll receive updates for %s here."
			return nil
		}

		message = "You're already receiving notifications for %s here!"
		return nil
	}

	if alreadyExists {
		hookID, err := h.db.GetHookIDForRepo(msg.ConvID, args[0])
		if err != nil {
			return fmt.Errorf("error getting hook ID for subscription: %s", err)
		}

		_, err = client.Projects.DeleteProjectHook(args[0], int(hookID))
		if err != nil {
			return fmt.Errorf("error deleting webhook: %s", err)
		}

		err = h.db.DeleteSubscriptionsForRepo(msg.ConvID, args[0])
		if err != nil {
			return fmt.Errorf("error deleting subscriptions: %s", err)
		}
		message = "Okay, you won't receive updates for %s here."
		return nil
	}

	message = "You aren't subscribed to updates for %s!"
	return nil
}

func (h *Handler) handleSubscribeToBranch(cmd string, msg chat1.MsgSummary, create bool, client *gitlab.Client) (err error) {
	toks, err := shellquote.Split(cmd)
	if err != nil {
		return fmt.Errorf("error splitting command: %s", err)
	}
	args := toks[2:]
	var message string
	defer func() {
		if message != "" {
			_, err = h.kbc.SendMessageByConvID(msg.ConvID, fmt.Sprintf(message, args[0], args[1]))
			if err != nil {
				err = fmt.Errorf("error sending message: %s", err)
			}
		}
	}()

	if len(args) < 2 {
		return fmt.Errorf("bad args for subscribe to branch: %v", args)
	}

	project, err := getProject(args[0], client)
	if err != nil {
		return fmt.Errorf("error getting gitlab project: %s", err)
	}
	var defaultBranch = project.DefaultBranch

	if exists, err := h.db.GetSubscriptionExists(msg.ConvID, args[0], defaultBranch); !exists {
		if err != nil {
			return fmt.Errorf("error getting subscription: %s", err)
		}
		var message string
		if create {
			message = fmt.Sprintf("You aren't subscribed to updates yet!\nSend this first: `!github subscribe %s`", args[0])
		} else {
			message = fmt.Sprintf("You aren't subscribed to notifications for %s!", args[0])
		}
		_, err := h.kbc.SendMessageByConvID(msg.ConvID, message)
		if err != nil {
			return fmt.Errorf("Error sending message: %s", err)
		}
		return nil
	}

	if create {
		hookID, err := h.db.GetHookIDForRepo(msg.ConvID, args[0])
		if err != nil {
			return fmt.Errorf("error getting hook ID for subscription: %s", err)
		}

		err = h.db.CreateSubscription(msg.ConvID, args[0], args[1], hookID)
		if err != nil {
			return fmt.Errorf("error creating subscription: %s", err)
		}

		message = "Now subscribed to commits on %s/%s."
		return nil
	}
	err = h.db.DeleteSubscription(msg.ConvID, args[0], args[1])
	if err != nil {
		return fmt.Errorf("error deleting subscription: %s", err)
	}

	message = "Okay, you won't receive notifications for commits in %s/%s."
	return nil
}