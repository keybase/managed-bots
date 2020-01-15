package githubbot

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v28/github"
	"github.com/kballard/go-shellquote"
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
	welcomeMsg := "Hi! I can notify you whenever something happens on a GitHub repository. To get started, set up a repository by sending `!github subscribe <username/repo>`"
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
	if !strings.HasPrefix(cmd, "!github") {
		h.Debug("ignoring non-command message")
		return nil
	}

	if strings.HasPrefix(cmd, "!github mentions") {
		// handle user preferences without needing oauth
		return h.handleMentionPref(cmd, msg)
	}

	identifier := base.IdentifierFromMsg(msg)
	tc, err := base.GetOAuthClient(identifier, msg, h.kbc, h.requests, h.config, h.db,
		base.GetOAuthOpts{
			AuthMessageTemplate: "Visit %s\n to authorize me to set up GitHub notifications.",
		})
	if err != nil || tc == nil {
		return err
	}

	client := github.NewClient(tc)

	switch {
	case strings.HasPrefix(cmd, "!github subscribe"):
		return h.handleSubscribe(cmd, msg, true, client)
	case strings.HasPrefix(cmd, "!github unsubscribe"):
		return h.handleSubscribe(cmd, msg, false, client)
	case strings.HasPrefix(cmd, "!github watch"):
		return h.handleWatch(cmd, msg.ConvID, true, client)
	case strings.HasPrefix(cmd, "!github unwatch"):
		return h.handleWatch(cmd, msg.ConvID, false, client)
	}
	return nil
}

func (h *Handler) handleSubscribe(cmd string, msg chat1.MsgSummary, create bool, client *github.Client) (err error) {
	toks, err := shellquote.Split(cmd)
	if err != nil {
		return fmt.Errorf("error splitting command: %s", err)
	}
	args := toks[2:]
	if len(args) < 1 {
		return fmt.Errorf("bad args for subscribe: %v", args)
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

	defaultBranch, err := getDefaultBranch(args[0], client)
	if err != nil {
		return fmt.Errorf("error getting default branch: %s", err)
	}
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
			hook, res, err := client.Repositories.CreateHook(context.TODO(), parsedRepo[0], parsedRepo[1], &github.Hook{
				Config: map[string]interface{}{
					"url":          h.httpPrefix + "/githubbot/webhook",
					"content_type": "json",
					"secret":       makeSecret(args[0], msg.ConvID, h.secret),
				},
				Events: []string{"*"},
			})

			if err != nil {
				if res.StatusCode != http.StatusNotFound {
					return fmt.Errorf("error: %s", err)
				}
				message = "I couldn't subscribe to updates on %s, do you have the right permissions?"
				return nil
			}
			err = h.db.CreateSubscription(msg.ConvID, args[0], defaultBranch, hook.GetID())
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

		_, err = client.Repositories.DeleteHook(context.TODO(), parsedRepo[0], parsedRepo[1], hookID)
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

func (h *Handler) handleWatch(cmd string, convID chat1.ConvIDStr, create bool, client *github.Client) error {
	toks, err := shellquote.Split(cmd)
	if err != nil {
		return fmt.Errorf("error splitting command: %s", err)
	}
	args := toks[2:]
	var message string
	defer func() {
		if message != "" {
			_, err = h.kbc.SendMessageByConvID(convID, fmt.Sprintf(message, args[0], args[1]))
			if err != nil {
				err = fmt.Errorf("error sending message: %s", err)
			}
		}
	}()

	if len(args) < 2 {
		return fmt.Errorf("bad args for watch: %v", args)
	}
	defaultBranch, err := getDefaultBranch(args[0], client)
	if err != nil {
		return fmt.Errorf("error getting default branch: %s", err)
	}

	if exists, err := h.db.GetSubscriptionExists(convID, args[0], defaultBranch); !exists {
		if err != nil {
			return fmt.Errorf("error getting subscription: %s", err)
		}
		_, err := h.kbc.SendMessageByConvID(convID, fmt.Sprintf("You aren't subscribed to notifications for %s!", args[0]))
		if err != nil {
			return fmt.Errorf("Error sending message: %s", err)
		}
		return nil
	}

	if create {
		hookID, err := h.db.GetHookIDForRepo(convID, args[0])
		if err != nil {
			return fmt.Errorf("error getting hook ID for subscription: %s", err)
		}

		err = h.db.CreateSubscription(convID, args[0], args[1], hookID)
		if err != nil {
			return fmt.Errorf("error creating subscription: %s", err)
		}

		message = "Now watching for commits on %s/%s."
		return nil
	}
	err = h.db.DeleteSubscription(convID, args[0], args[1])
	if err != nil {
		return fmt.Errorf("error deleting subscription: %s", err)
	}

	message = "Okay, you won't receive notifications for commits in %s/%s."
	return nil
}

// user preferences
func (h *Handler) handleMentionPref(cmd string, msg chat1.MsgSummary) (err error) {
	var message string
	defer func() {
		if message != "" {
			_, err = h.kbc.SendMessageByConvID(msg.ConvID, message)
			if err != nil {
				err = fmt.Errorf("error sending message: %s", err)
			}
		}
	}()

	toks, err := shellquote.Split(cmd)
	if err != nil {
		return fmt.Errorf("error splitting command: %s", err)
	}
	args := toks[2:]
	if len(args) != 1 || (args[0] != "disable" && args[0] != "enable") {
		message = "I don't understand! Try `!github mentions disable` or `!github mentions enable`."
		return nil
	}

	allowMentions := args[0] == "enable"
	err = h.db.SetUserPreferences(msg.Sender.Username, &UserPreferences{Mention: allowMentions})
	if err != nil {
		return fmt.Errorf("error setting user preference: %s", err)
	}

	if allowMentions {
		message = "Okay, you'll be mentioned in GitHub events involving your linked GitHub account."
	} else {
		message = "Okay, you won't be mentioned in future GitHub events."
	}

	return

}
