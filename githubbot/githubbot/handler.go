package githubbot

import (
	"fmt"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type Handler struct {
	kbc        *kbchat.API
	httpSrv    *HTTPSrv
	httpPrefix string
}

func NewHandler(kbc *kbchat.API, httpSrv *HTTPSrv, httpPrefix string) *Handler {
	return &Handler{
		kbc:        kbc,
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
	case strings.HasPrefix(cmd, "!poll"):
		// h.handlePoll(cmd, msg.ConvID, msg.Id)
	case cmd == "login":
		// h.handleLogin(msg.Channel.Name, msg.Sender.Username)
	default:
		h.debug("ignoring unknown command %s", cmd)
	}
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
