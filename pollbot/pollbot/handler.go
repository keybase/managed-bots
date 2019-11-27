package pollbot

import (
	"fmt"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type Handler struct {
	kbc *kbchat.API
}

func NewHandler(kbc *kbchat.API) *Handler {
	return &Handler{
		kbc: kbc,
	}
}

func (h *Handler) debug(msg string, args ...interface{}) {
	fmt.Printf("Handler: "+msg+"\n", args...)
}

func (h *Handler) chatDebug(convID, msg string, args ...interface{}) {
	s.debug(msg, args...)
	if _, err := s.kbc.SendMessageByConvID(convID, msg, args...); err != nil {
		s.debug("chatDebug: failed to send error message: %s", err)
	}
}

func (h *Handler) Listen() {
	sub, err := h.kbc.ListenForNewTextMessages()
	if err != nil {
		return err
	}
	h.debug("startup success, listening for messages...")
	for {
		msg, err := sub.Read()
		if err != nil {
			s.debug("Read() error: %s", err.Error())
			continue
		}
		h.handleCommand(msg.Message)
	}
}

func (h *Handler) handlePoll(cmd, convID string, msgID chat1.MessageID) {

}

func (h *Handler) handleCommand(msg chat1.MsgSummary) {
	if msg.Content.Text == nil {
		h.debug("skipping non-text message")
		return
	}
	cmd := strings.Trim(msg.Content.Text.Body, " ")
	switch {
	case strings.HasPrefix(cmd, "!poll"):
		h.handlePoll(cmd, msg.ConvID, msg.Id)
	default:
		h.debug("ignoring unknown command")
	}
}
