package pagerdutybot

import (
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type Handler struct {
	*base.DebugOutput

	kbc        *kbchat.API
	db         *DB
	httpSrv    *HTTPSrv
	httpPrefix string
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig,
	httpSrv *HTTPSrv, db *DB, httpPrefix string) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		kbc:         kbc,
		db:          db,
		httpSrv:     httpSrv,
		httpPrefix:  httpPrefix,
	}
}

func (h *Handler) formURL(id string) string {
	return fmt.Sprintf("%s/pagerdutybot/%s", h.httpPrefix, id)
}

func (h *Handler) checkAdmin(msg chat1.MsgSummary) (bool, error) {
	ok, err := base.IsAdmin(h.kbc, msg)
	if err != nil {
		return false, fmt.Errorf("handleCreate: failed to check admin: %s", err)
	}
	if !ok {
		h.ChatDebug(msg.ConvID, "only admins can administer webhooks")
		return false, nil
	}
	return true, nil
}

func (h *Handler) handleIntegrate(cmd string, msg chat1.MsgSummary) error {
	convID := msg.ConvID
	if isAdmin, err := h.checkAdmin(msg); err != nil || !isAdmin {
		return err
	}

	id, err := h.db.Create(convID)
	if err != nil {
		return fmt.Errorf("handleIntegrate: failed to create webhook: %s", err)
	}
	if _, err := h.kbc.SendMessageByTlfName(msg.Sender.Username, "%s", h.formURL(id)); err != nil {
		h.Debug("handleIntegrate: failed to send hook: %s", err)
	}
	h.ChatEcho(convID, "Success! New URL sent to @%s", msg.Sender.Username)
	return nil
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "Thank you for adding me! I am here to integrate your PagerDuty setup into Keybase. Try `!pagerduty integrate` to get started."
	return base.HandleNewTeam(h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		return nil
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!pagerduty integrate"):
		return h.handleIntegrate(cmd, msg)
	}
	return nil
}
