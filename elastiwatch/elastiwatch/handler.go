package elastiwatch

import (
	"errors"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type Handler struct {
	*base.DebugOutput

	kbc     *kbchat.API
	httpSrv *HTTPSrv
	db      *DB
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig,
	httpSrv *HTTPSrv, db *DB) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		kbc:         kbc,
		httpSrv:     httpSrv,
		db:          db,
	}
}

func (h *Handler) handleDefer(cmd string) error {
	return errors.New("not implemented")
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		return nil
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!elastiwatch defer"):
		return h.handleDefer(cmd)
	}
	return nil
}

func (h *Handler) HandleNewConv(chat1.ConvSummary) error {
	return nil
}
