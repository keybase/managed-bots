package githubbot

import (
	"context"
	"fmt"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type Handler struct {
	kbc      *kbchat.API
	db       *DB
	httpSrv  *HTTPSrv
	httpAddr string
	secret   string
	ctx      context.Context
}

func NewHandler(kbc *kbchat.API, db *DB, httpSrv *HTTPSrv, httpAddr string, secret string, ctx context.Context) *Handler {
	return &Handler{
		kbc:      kbc,
		db:       db,
		httpSrv:  httpSrv,
		httpAddr: httpAddr,
		secret:   secret,
		ctx:      ctx,
	}
}

func (h *Handler) debug(msg string, args ...interface{}) {
	fmt.Printf("Handler: "+msg+"\n", args...)
}

func (h *Handler) chatDebug(convID, msg string, args ...interface{}) {
	h.debug(msg, args...)
	if _, err := h.kbc.SendMessageByConvID(convID, "Something went wrong!"); err != nil {
		h.debug("chatDebug: failed to send error message: %s", err)
	}
}

func (h *Handler) handleCommand(msg chat1.MsgSummary) {
	if msg.Content.Text == nil {
		h.debug("skipping non-text message")
		return
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)

	if !strings.HasPrefix(cmd, "!github") {
		h.debug("ignoring non-command message")
		return
	}

	isAdmin, err := h.isAdmin(msg)
	if err != nil {
		h.chatDebug(msg.ConvID, "Error getting admin status: %s", err)
		return
	}
	if !isAdmin {
		h.kbc.SendMessageByConvID(msg.ConvID, "You must be an admin to configure me for a team!")
		return
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
		h.debug("ignoring unknown command")
	}
}

func (h *Handler) handleSubscribe(cmd string, msg chat1.MsgSummary, create bool) {
	toks, err := shellquote.Split(cmd)
	args := toks[2:]
	if len(args) < 1 {
		h.chatDebug(msg.ConvID, "bad args for subscribe: %s", args)
		return
	}

	var message string
	defaultBranch, err := getDefaultBranch(h.ctx, args[0])
	if err != nil {
		h.chatDebug(msg.ConvID, "error getting default branch: %s", err)
		return
	}
	alreadyExists, err := h.db.GetSubscriptionForRepoExists(msg.ConvID, args[0])
	if err != nil {
		h.chatDebug(msg.ConvID, "error checking subscription: %s", err)
		return
	}
	if create {
		if !alreadyExists {
			err = h.db.CreateSubscription(msg.ConvID, args[0], defaultBranch)
			if err != nil {
				h.chatDebug(msg.ConvID, fmt.Sprintf("Error creating subscription: %s", err))
				return
			}

			// setting up phase - send instructions
			h.kbc.SendMessageByTlfName(msg.Sender.Username, formatSetupInstructions(args[0], h.httpAddr, h.secret))
			if strings.HasPrefix(msg.Channel.Name, msg.Sender.Username+",") {
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
				h.chatDebug(msg.ConvID, fmt.Sprintf("Error deleting subscriptions: %s", err))
				return
			}
			message = "Okay, you won't receive updates for %s here."
		} else {
			message = "You aren't subscribed to updates for %s!"
		}
	}

	h.kbc.SendMessageByConvID(msg.ConvID, fmt.Sprintf(message, args[0]))

}

func (h *Handler) handleWatch(cmd string, convID string, create bool) {
	toks, err := shellquote.Split(cmd)
	args := toks[2:]
	var message string

	if len(args) < 2 {
		h.chatDebug(convID, "bad args for watch: %s", args)
		return
	}
	defaultBranch, err := getDefaultBranch(h.ctx, args[0])
	if err != nil {
		h.chatDebug(convID, "error getting default branch: %s", err)
		return
	}
	if exists, err := h.db.GetSubscriptionExists(convID, args[0], defaultBranch); !exists {
		if err != nil {
			h.chatDebug(convID, fmt.Sprintf("Error getting subscription: %s", err))
			return
		}
		h.kbc.SendMessageByConvID(convID, fmt.Sprintf("You aren't subscribed to notifications for %s!", args[0]))
		return
	}
	if create {
		err = h.db.CreateSubscription(convID, args[0], args[1])
		if err != nil {
			h.chatDebug(convID, fmt.Sprintf("Error creating subscription: %s", err))
			return
		}
		message = "Now watching for commits on %s/%s."
	} else {
		err = h.db.DeleteSubscription(convID, args[0], args[1])
		if err != nil {
			h.chatDebug(convID, fmt.Sprintf("Error deleting subscription: %s", err))
			return
		}
		message = "Okay, you wont receive notifications for commits in %s/%s."
	}
	h.kbc.SendMessageByConvID(convID, fmt.Sprintf(message, args[0], args[1]))
}

func (h *Handler) Listen() error {
	sub, err := h.kbc.ListenForNewTextMessages()
	if err != nil {
		h.debug("Listen: failed to listen: %s", err)
		return err
	}
	h.debug("startup success, listening for messages...")
	for {
		msg, err := sub.Read()
		if err != nil {
			h.debug("Listen: Read() error: %s", err)
			continue
		}
		h.handleCommand(msg.Message)
	}
}

func (h *Handler) isAdmin(msg chat1.MsgSummary) (bool, error) {
	switch msg.Channel.MembersType {
	case "team": // make sure the member is an admin or owner
	default: // authorization is per user so let anything through
		return true, nil
	}

	res, err := h.kbc.ListMembersOfTeam(msg.Channel.Name)
	if err != nil {
		return false, err
	}
	adminLike := append(res.Owners, res.Admins...)
	for _, member := range adminLike {
		if member.Username == msg.Sender.Username {
			return true, nil
		}
	}
	return false, nil
}
