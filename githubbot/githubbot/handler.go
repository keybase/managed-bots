package githubbot

import (
	"fmt"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type Handler struct {
	kbc        *kbchat.API
	db         *DB
	httpSrv    *HTTPSrv
	httpPrefix string
}

func NewHandler(kbc *kbchat.API, db *DB, httpSrv *HTTPSrv, httpPrefix string) *Handler {
	return &Handler{
		kbc:        kbc,
		db:         db,
		httpSrv:    httpSrv,
		httpPrefix: httpPrefix,
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
	cmd := strings.Trim(msg.Content.Text.Body, " ")
	switch {
	case strings.HasPrefix(cmd, "!github subscribe"):
		h.handleSubscribe(cmd, msg.ConvID)
	default:
		h.debug("ignoring unknown command")
	}
}

func (h *Handler) handleSubscribe(cmd string, convID string) {
	toks, err := shellquote.Split(cmd)
	args := toks[1:]
	if len(args) < 2 {
		h.chatDebug(convID, "must specify a prompt and at least one option")
	}
	err = h.db.CreateSubscription(convID, args[0])
	if err != nil {
		h.chatDebug(convID, fmt.Sprintf("Sorry, something went wrong."))
		return
	}
	h.kbc.SendMessageByConvID(convID, fmt.Sprintf("Ok, you'll now receive updates for %s here!", args[0]))
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
