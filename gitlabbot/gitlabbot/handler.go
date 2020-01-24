package gitlabbot

import (
	"fmt"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type Handler struct {
	*base.DebugOutput

	kbc        *kbchat.API
	db         *DB
	httpPrefix string
	secret     string
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig,
	db *DB, httpPrefix string, secret string) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		kbc:         kbc,
		db:          db,
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
		return nil
	}

	cmd := strings.ToLower(strings.TrimSpace(msg.Content.Text.Body))
	if !strings.HasPrefix(cmd, "!gitlab") {
		return nil
	}

	switch {
	case strings.HasPrefix(cmd, "!gitlab subscribe"):
		return h.handleSubscribe(cmd, msg, true)
	case strings.HasPrefix(cmd, "!gitlab unsubscribe"):
		return h.handleSubscribe(cmd, msg, false)
	}
	return nil
}

func (h *Handler) handleSubscribe(cmd string, msg chat1.MsgSummary, create bool) (err error) {
	toks, userErr, err := base.SplitTokens(cmd)
	if err != nil {
		return err
	} else if userErr != "" {
		h.ChatEcho(msg.ConvID, userErr)
		return nil
	}
	args := toks[2:]
	if len(args) < 1 {
		return fmt.Errorf("bad args for subscribe: %v", args)
	}

	// Check if command is subscribing to a branch
	if len(toks) == 4 {
		return h.handleSubscribeToBranch(cmd, msg, create)
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
			defaultBranch := "master"

			err = h.db.CreateSubscription(msg.ConvID, args[0], defaultBranch, base.IdentifierFromMsg(msg))
			if err != nil {
				return fmt.Errorf("error creating subscription: %s", err)
			}

			_, err = h.kbc.SendMessageByTlfName(msg.Sender.Username, formatSetupInstructions(args[0], msg, h.httpPrefix, h.secret))
			if err != nil {
				return fmt.Errorf("error sending message: %s", err)
			}

			return nil
		}

		message = "You're already receiving notifications for %s here!"
		return nil
	}

	if alreadyExists {
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

func (h *Handler) handleSubscribeToBranch(cmd string, msg chat1.MsgSummary, create bool) (err error) {
	toks, userErr, err := base.SplitTokens(cmd)
	if err != nil {
		return err
	} else if userErr != "" {
		h.ChatEcho(msg.ConvID, userErr)
		return nil
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

	defaultBranch := "master"

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
		err = h.db.CreateSubscription(msg.ConvID, args[0], args[1], base.IdentifierFromMsg(msg))
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
