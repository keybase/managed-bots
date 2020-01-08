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

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		h.Debug("skipping non-text message")
		return nil
	}

	cmd := strings.TrimSpace(msg.Content.Text.Body)
	if !strings.HasPrefix(cmd, "!github") {
		h.Debug("ignoring non-command message")
		return nil
	}

	tc, isAdmin, err := h.getOAuthClient(msg)
	if err != nil {
		return err
	}
	if tc == nil {
		if !isAdmin {
			_, err = h.kbc.SendMessageByConvID(msg.ConvID, "You have must be an admin to authorize me for a team!")
			return err
		}
		// If we are in a 1-1 conv directly or as a bot user with the sender,
		// skip this message.
		if msg.Channel.MembersType == "team" || !(msg.Sender.Username == msg.Channel.Name || len(strings.Split(msg.Channel.Name, ",")) == 2) {
			_, err = h.kbc.SendMessageByConvID(msg.ConvID,
				"OK! I've sent a message to @%s to authorize me.", msg.Sender.Username)
			return err
		}
		return nil
	}

	client := github.NewClient(tc)

	switch {
	case strings.HasPrefix(cmd, "!github subscribe"):
		h.handleSubscribe(cmd, msg, true, client)
	case strings.HasPrefix(cmd, "!github unsubscribe"):
		h.handleSubscribe(cmd, msg, false, client)
	case strings.HasPrefix(cmd, "!github watch"):
		h.handleWatch(cmd, msg.ConvID, true, client)
	case strings.HasPrefix(cmd, "!github unwatch"):
		h.handleWatch(cmd, msg.ConvID, false, client)
	default:
		h.Debug("ignoring unknown command")
	}
	return nil
}

func (h *Handler) handleSubscribe(cmd string, msg chat1.MsgSummary, create bool, client *github.Client) {
	toks, err := shellquote.Split(cmd)
	if err != nil {
		h.ChatDebug(msg.ConvID, "error splitting command: %s", err)
		return
	}
	args := toks[2:]
	if len(args) < 1 {
		h.ChatDebug(msg.ConvID, "bad args for subscribe: %s", args)
		return
	}

	var message string
	defer func() {
		if message != "" {
			_, err = h.kbc.SendMessageByConvID(msg.ConvID, fmt.Sprintf(message, args[0]))
			if err != nil {
				h.ChatDebug(msg.ConvID, "Error sending message: %s", err)
				return
			}
		}
	}()

	defaultBranch, err := getDefaultBranch(args[0], client)
	if err != nil {
		h.ChatDebug(msg.ConvID, "error getting default branch: %s", err)
		return
	}
	alreadyExists, err := h.db.GetSubscriptionForRepoExists(base.ShortConvID(msg.ConvID), args[0])
	if err != nil {
		h.ChatDebug(msg.ConvID, "error checking subscription: %s", err)
		return
	}

	parsedRepo := strings.Split(args[0], "/")
	if len(parsedRepo) != 2 {
		h.ChatDebug(msg.ConvID, fmt.Sprintf("invalid repo: %s", args[0]))
		return
	}
	if create {
		if !alreadyExists {
			hook, res, err := client.Repositories.CreateHook(context.TODO(), parsedRepo[0], parsedRepo[1], &github.Hook{
				Config: map[string]interface{}{
					"url":          h.httpPrefix + "/githubbot/webhook",
					"content_type": "json",
					"secret":       makeSecret(args[0], base.ShortConvID(msg.ConvID), h.secret),
				},
				Events: []string{"*"},
			})

			if err != nil {
				if res.StatusCode != http.StatusNotFound {
					h.ChatDebug(msg.ConvID, fmt.Sprintf("error: %s", err))
					return
				}
				message = "I couldn't subscribe to updates on %s, do you have the right permissions?"
				return
			}
			err = h.db.CreateSubscription(base.ShortConvID(msg.ConvID), args[0], defaultBranch, hook.GetID())
			if err != nil {
				h.ChatDebug(msg.ConvID, fmt.Sprintf("error creating subscription: %s", err))
				return
			}
			message = "Okay, you'll receive updates for %s here."
			return
		}

		message = "You're already receiving notifications for %s here!"
		return
	}

	if alreadyExists {
		hookID, err := h.db.GetHookIDForRepo(base.ShortConvID(msg.ConvID), args[0])
		if err != nil {
			h.ChatDebug(msg.ConvID, fmt.Sprintf("Error getting hook ID for subscription: %s", err))
			return
		}

		_, err = client.Repositories.DeleteHook(context.TODO(), parsedRepo[0], parsedRepo[1], hookID)
		if err != nil {
			h.ChatDebug(msg.ConvID, fmt.Sprintf("Error deleting webhook: %s", err))
			return
		}

		err = h.db.DeleteSubscriptionsForRepo(base.ShortConvID(msg.ConvID), args[0])
		if err != nil {
			h.ChatDebug(msg.ConvID, fmt.Sprintf("Error deleting subscriptions: %s", err))
			return
		}
		message = "Okay, you won't receive updates for %s here."
		return
	}

	message = "You aren't subscribed to updates for %s!"
}

func (h *Handler) handleWatch(cmd string, convID string, create bool, client *github.Client) {
	toks, err := shellquote.Split(cmd)
	if err != nil {
		h.Debug("error splitting command: %s", err)
		return
	}
	args := toks[2:]
	var message string
	defer func() {
		if message != "" {
			_, err = h.kbc.SendMessageByConvID(convID, fmt.Sprintf(message, args[0], args[1]))
			if err != nil {
				h.ChatDebug(convID, "Error sending message: %s", err)
				return
			}
		}
	}()

	if len(args) < 2 {
		h.ChatDebug(convID, "bad args for watch: %s", args)
		return
	}
	defaultBranch, err := getDefaultBranch(args[0], client)
	if err != nil {
		h.ChatDebug(convID, "error getting default branch: %s", err)
		return
	}

	if exists, err := h.db.GetSubscriptionExists(base.ShortConvID(convID), args[0], defaultBranch); !exists {
		if err != nil {
			h.ChatDebug(convID, fmt.Sprintf("Error getting subscription: %s", err))
			return
		}
		_, err := h.kbc.SendMessageByConvID(convID, fmt.Sprintf("You aren't subscribed to notifications for %s!", args[0]))
		if err != nil {
			h.ChatDebug(convID, "Error sending message: %s", err)
			return
		}
		return
	}

	if create {
		hookID, err := h.db.GetHookIDForRepo(base.ShortConvID(convID), args[0])
		if err != nil {
			h.ChatDebug(convID, fmt.Sprintf("Error getting hook ID for subscription: %s", err))
			return
		}

		err = h.db.CreateSubscription(base.ShortConvID(convID), args[0], args[1], hookID)
		if err != nil {
			h.ChatDebug(convID, fmt.Sprintf("Error creating subscription: %s", err))
			return
		}

		message = "Now watching for commits on %s/%s."
		return
	}
	err = h.db.DeleteSubscription(base.ShortConvID(convID), args[0], args[1])
	if err != nil {
		h.ChatDebug(convID, fmt.Sprintf("Error deleting subscription: %s", err))
		return
	}

	message = "Okay, you won't receive notifications for commits in %s/%s."
}

func (h *Handler) getOAuthClient(msg chat1.MsgSummary) (*http.Client, bool, error) {
	token, err := h.db.GetToken(base.IdentifierFromMsg(msg))
	if err != nil {
		return nil, false, err
	}
	// We need to request new authorization
	if token == nil {
		if isAdmin, err := base.IsAdmin(h.kbc, msg); err != nil || !isAdmin {
			return nil, isAdmin, err
		}

		state, err := base.MakeRequestID()
		if err != nil {
			return nil, false, err
		}
		h.requests.Lock()
		h.requests.Map[state] = msg
		h.requests.Unlock()
		authURL := h.config.AuthCodeURL(string(state), oauth2.ApprovalForce)
		// strip protocol to skip unfurl prompt
		authURL = strings.TrimPrefix(authURL, "https://")
		_, err = h.kbc.SendMessageByTlfName(msg.Sender.Username, "Visit %s\n to authorize me to set up GitHub notifications.", authURL)
		return nil, true, err
	}
	return h.config.Client(context.Background(), token), false, nil
}
