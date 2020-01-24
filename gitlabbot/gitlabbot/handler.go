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

	repo := args[0]
	alreadyExists, err := h.db.GetSubscriptionForRepoExists(msg.ConvID, repo)
	if err != nil {
		return fmt.Errorf("error checking subscription: %s", err)
	}

	parsedRepo := strings.Split(repo, "/")
	if len(parsedRepo) != 2 {
		return fmt.Errorf("invalid repo: %s", repo)
	}
	if create {
		if !alreadyExists {
			defaultBranch := "master"
			err = h.db.CreateSubscription(msg.ConvID, repo, defaultBranch, base.IdentifierFromMsg(msg))
			if err != nil {
				return fmt.Errorf("error creating subscription: %s", err)
			}
			_, err = h.kbc.SendMessageByTlfName(msg.Sender.Username, formatSetupInstructions(repo, msg, h.httpPrefix, h.secret))
			if err != nil {
				return fmt.Errorf("error sending message: %s", err)
			}
			if !base.IsDirectPrivateMessage(msg) {
				h.ChatEcho(msg.ConvID, "OK! I've sent a message to @%s to authorize me.", msg.Sender.Username)
			}
			return nil
		}

		h.ChatEcho(msg.ConvID, "You're already receiving notifications for `%s` here!", repo)
		return nil
	}

	if alreadyExists {
		err = h.db.DeleteSubscriptionsForRepo(msg.ConvID, repo)
		if err != nil {
			return fmt.Errorf("error deleting subscriptions: %s", err)
		}
		h.ChatEcho(msg.ConvID, "Okay, you won't receive updates for `%s` here.", repo)
		return nil
	}

	h.ChatEcho(msg.ConvID, "You aren't subscribed to updates for `%s`!", repo)
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
	if len(args) < 2 {
		return fmt.Errorf("bad args for subscribe to branch: %v", args)
	}

	defaultBranch := "master"
	repo := args[0]
	branch := args[1]
	exists, err := h.db.GetSubscriptionExists(msg.ConvID, repo, defaultBranch)
	if err != nil {
		return err
	} else if !exists {
		if create {
			_, err = h.kbc.SendMessageByTlfName(msg.Sender.Username, formatSetupInstructions(repo, msg, h.httpPrefix, h.secret))
			if err != nil {
				return fmt.Errorf("error sending message: %s", err)
			}
			if !base.IsDirectPrivateMessage(msg) {
				h.ChatEcho(msg.ConvID, "OK! I've sent a message to @%s to authorize me.", msg.Sender.Username)
			}
		} else {
			h.ChatEcho(msg.ConvID, "You aren't subscribed to notifications for `%s`!", repo)
		}
		return nil
	}

	if create {
		err = h.db.CreateSubscription(msg.ConvID, repo, branch, base.IdentifierFromMsg(msg))
		if err != nil {
			return fmt.Errorf("error creating subscription: %s", err)
		}

		if exists {
			h.ChatEcho(msg.ConvID, "Now subscribed to commits on `%s/%s`.", repo, branch)
		}
		return nil
	}

	err = h.db.DeleteSubscription(msg.ConvID, repo, branch)
	if err != nil {
		return fmt.Errorf("error deleting subscription: %s", err)
	}

	h.ChatEcho(msg.ConvID, "Okay, you won't receive notifications for commits in `%s/%s`.", repo, branch)
	return nil
}
