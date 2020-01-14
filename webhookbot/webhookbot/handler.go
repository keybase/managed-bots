package webhookbot

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

func NewHandler(kbc *kbchat.API, httpSrv *HTTPSrv, db *DB, httpPrefix string) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", kbc),
		kbc:         kbc,
		db:          db,
		httpSrv:     httpSrv,
		httpPrefix:  httpPrefix,
	}
}

func (h *Handler) formURL(id string) string {
	return fmt.Sprintf("%s/webhookbot/%s", h.httpPrefix, id)
}

func (h *Handler) checkAdmin(msg chat1.MsgSummary) (bool, error) {
	ok, err := base.IsAdmin(h.kbc, msg)
	if err != nil {
		return false, fmt.Errorf("handleCreate: failed to check admin: %s", err)
	}
	if !ok {
		return false, fmt.Errorf("only admins can administer webhooks")
	}
	return true, nil
}

func (h *Handler) handleRemove(cmd string, msg chat1.MsgSummary) error {
	convID := msg.ConvID
	toks := strings.Split(cmd, " ")
	if len(toks) != 3 {
		return fmt.Errorf("invalid number of arguments, must specify a name")
	}
	if isAdmin, err := h.checkAdmin(msg); err != nil || !isAdmin {
		return err
	}
	name := toks[2]
	if err := h.db.Remove(name, convID); err != nil {
		return fmt.Errorf("handleRemove: failed to remove webhook: %s", err)
	}
	h.ChatEcho(convID, "Success!")
	return nil
}

func (h *Handler) handleList(cmd string, msg chat1.MsgSummary) error {
	convID := msg.ConvID
	hooks, err := h.db.List(convID)
	if err != nil {
		return fmt.Errorf("handleList: failed to list hook: %s", err)
	}
	if isAdmin, err := h.checkAdmin(msg); err != nil || !isAdmin {
		return err
	}

	if len(hooks) == 0 {
		h.ChatEcho(convID, "No hooks in this conversation")
		return nil
	}
	var body string
	for _, hook := range hooks {
		body += fmt.Sprintf("%s, %s\n", hook.name, h.formURL(hook.id))
	}
	if _, err := h.kbc.SendMessageByTlfName(msg.Sender.Username, body); err != nil {
		h.Debug(convID, "handleList: failed to send hook: %s", err)
	}
	h.ChatEcho(convID, "List sent to @%s", msg.Sender.Username)
	return nil
}

func (h *Handler) handleCreate(cmd string, msg chat1.MsgSummary) error {
	convID := msg.ConvID
	toks := strings.Split(cmd, " ")
	if len(toks) != 3 {
		return fmt.Errorf("invalid number of arguments, must specify a name")
	}
	if isAdmin, err := h.checkAdmin(msg); err != nil || !isAdmin {
		return err
	}

	name := toks[2]
	id, err := h.db.Create(name, convID)
	if err != nil {
		return fmt.Errorf("handleCreate: failed to create webhook: %s", err)
	}
	if _, err := h.kbc.SendMessageByTlfName(msg.Sender.Username, "%s", h.formURL(id)); err != nil {
		h.Debug(convID, "handleCreate: failed to send hook: %s", err)
	}
	h.ChatEcho(convID, "Success! New URL sent to @%s", msg.Sender.Username)
	return nil
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "I can create generic webhooks into Keybase! Try `!webhook create` to get started."
	return base.HandleNewTeam(h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		h.Debug("skipping non-text message")
		return nil
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!webhook create"):
		return h.handleCreate(cmd, msg)
	case strings.HasPrefix(cmd, "!webhook list"):
		return h.handleList(cmd, msg)
	case strings.HasPrefix(cmd, "!webhook remove"):
		return h.handleRemove(cmd, msg)
	}
	return nil
}
