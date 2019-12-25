package triviabot

import (
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type Handler struct {
	*base.Handler

	kbc *kbchat.API
	db  *DB
}

var _ base.CommandHandler = (*Handler)(nil)

func NewHandler(kbc *kbchat.API, db *DB) *Handler {
	h := &Handler{
		kbc: kbc,
		db:  db,
	}
	h.Handler = base.NewHandler(kbc, h)
	return h
}

func (h *Handler) handleStart(cmd string, msg chat1.MsgSummary) {
	convID := msg.ConvID
	session := newSession(h.kbc, convID)
	if err := session.start(0); err != nil {
		h.ChatDebug(convID, "handleState: failed to start: %s", err)
	}
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		h.Debug("skipping non-text message")
		return nil
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!trivia start"):
		h.handleStart(cmd, msg)
	default:
		h.Debug("ignoring unknown command: %q", cmd)
	}
	return nil
}
