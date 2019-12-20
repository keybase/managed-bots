package githubbot

import (
	"fmt"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type Handler struct {
	*base.Handler

	kbc        *kbchat.API
	db         *DB
	httpSrv    *HTTPSrv
	httpPrefix string
	secret     string
}

var _ base.CommandHandler = (*Handler)(nil)

func NewHandler(kbc *kbchat.API, db *DB, httpSrv *HTTPSrv, httpPrefix string, secret string) *Handler {
	return &Handler{
		kbc:        kbc,
		db:         db,
		httpSrv:    httpSrv,
		httpPrefix: httpPrefix,
		secret:     secret,
	}
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

	isAdmin, err := h.IsAdmin(msg)
	if err != nil {
		h.ChatDebug(msg.ConvID, "Error getting admin status: %s", err)
		return nil
	} else if !isAdmin {
		_, err = h.kbc.SendMessageByConvID(msg.ConvID, "You must be an admin to configure me for a team!")
		if err != nil {
			return err
		}
		return nil
	}

	switch {
	case strings.HasPrefix(cmd, "!github subscribe"):
		h.handleSubscribe(cmd, msg, true)
	case strings.HasPrefix(cmd, "!github unsubscribe"):
		h.handleSubscribe(cmd, msg, false)
	case strings.HasPrefix(cmd, "!github watch"):
		h.handleWatch(cmd, msg.ConvID, true)
	case strings.HasPrefix(cmd, "!github unwatch"):
		h.handleWatch(cmd, msg.ConvID, false)
	default:
		h.Debug("ignoring unknown command")
	}
	return nil
}

func (h *Handler) handleSubscribe(cmd string, msg chat1.MsgSummary, create bool) {
	toks, err := shellquote.Split(cmd)
	if err != nil {
		h.Debug("error splitting command: %s", err)
		return
	}
	args := toks[2:]
	if len(args) < 1 {
		h.ChatDebug(msg.ConvID, "bad args for subscribe: %s", args)
		return
	}

	var message string
	defaultBranch, err := getDefaultBranch(args[0])
	if err != nil {
		h.ChatDebug(msg.ConvID, "error getting default branch: %s", err)
		return
	}
	alreadyExists, err := h.db.GetSubscriptionForRepoExists(msg.ConvID, args[0])
	if err != nil {
		h.ChatDebug(msg.ConvID, "error checking subscription: %s", err)
		return
	}
	if create {
		if !alreadyExists {
			err = h.db.CreateSubscription(msg.ConvID, args[0], defaultBranch)
			if err != nil {
				h.ChatDebug(msg.ConvID, fmt.Sprintf("Error creating subscription: %s", err))
				return
			}

			// setting up phase - send instructions
			_, err = h.kbc.SendMessageByTlfName(msg.Sender.Username, formatSetupInstructions(args[0], h.httpPrefix, h.secret))
			if err != nil {
				h.ChatDebug(msg.ConvID, "Error sending message: %s", err)
				return
			}

			if msg.Channel.MembersType != "team" && (msg.Sender.Username == msg.Channel.Name || len(strings.Split(msg.Channel.Name, ",")) == 2) {
				// don't send add'l message if in a 1:1 convo with sender
				return
			}

			message = fmt.Sprintf("Okay! I've sent instructions to @%s to set up notifications on", msg.Sender.Username) + " %s."

		} else {
			message = "You're already receiving notifications for %s here!"
		}
	} else {
		if alreadyExists {
			err = h.db.DeleteSubscriptionsForRepo(msg.ConvID, args[0])
			if err != nil {
				h.ChatDebug(msg.ConvID, fmt.Sprintf("Error deleting subscriptions: %s", err))
				return
			}
			message = "Okay, you won't receive updates for %s here."
		} else {
			message = "You aren't subscribed to updates for %s!"
		}
	}

	_, err = h.kbc.SendMessageByConvID(msg.ConvID, fmt.Sprintf(message, args[0]))
	if err != nil {
		h.ChatDebug(msg.ConvID, "Error sending message: %s", err)
		return
	}

}

func (h *Handler) handleWatch(cmd string, convID string, create bool) {
	toks, err := shellquote.Split(cmd)
	if err != nil {
		h.Debug("error splitting command: %s", err)
		return
	}
	args := toks[2:]
	var message string

	if len(args) < 2 {
		h.ChatDebug(convID, "bad args for watch: %s", args)
		return
	}
	defaultBranch, err := getDefaultBranch(args[0])
	if err != nil {
		h.ChatDebug(convID, "error getting default branch: %s", err)
		return
	}
	if exists, err := h.db.GetSubscriptionExists(convID, args[0], defaultBranch); !exists {
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
		err = h.db.CreateSubscription(convID, args[0], args[1])
		if err != nil {
			h.ChatDebug(convID, fmt.Sprintf("Error creating subscription: %s", err))
			return
		}
		message = "Now watching for commits on %s/%s."
	} else {
		err = h.db.DeleteSubscription(convID, args[0], args[1])
		if err != nil {
			h.ChatDebug(convID, fmt.Sprintf("Error deleting subscription: %s", err))
			return
		}
		message = "Okay, you wont receive notifications for commits in %s/%s."
	}
	_, err = h.kbc.SendMessageByConvID(convID, fmt.Sprintf(message, args[0], args[1]))
	if err != nil {
		h.ChatDebug(convID, "Error sending message: %s", err)
		return
	}
}
