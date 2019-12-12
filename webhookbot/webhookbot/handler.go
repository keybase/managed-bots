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
	*base.Handler

	kbc        *kbchat.API
	db         *DB
	httpSrv    *HTTPSrv
	httpPrefix string
}

func NewHandler(kbc *kbchat.API, httpSrv *HTTPSrv, db *DB, httpPrefix string) *Handler {
	h := &Handler{
		kbc:        kbc,
		db:         db,
		httpSrv:    httpSrv,
		httpPrefix: httpPrefix,
	}
	h.Handler = base.NewHandler(kbc, h)
	return h
}

func (h *Handler) formURL(id string) string {
	return fmt.Sprintf("%s/webhookbot/%s", h.httpPrefix, id)
}

func (h *Handler) checkAdmin(msg chat1.MsgSummary) bool {
	ok, err := h.IsAdmin(msg)
	if err != nil {
		h.ChatDebug(msg.ConvID, "handleCreate: failed to check admin: %s", err)
		return false
	}
	if !ok {
		h.ChatEcho(msg.ConvID, "only admins can administer webhooks")
		return false
	}
	return true
}

func (h *Handler) handleRemove(cmd string, msg chat1.MsgSummary) {
	convID := msg.ConvID
	toks := strings.Split(cmd, " ")
	if len(toks) != 3 {
		h.ChatDebug(convID, "invalid number of arguments, must specify a name")
		return
	}
	if !h.checkAdmin(msg) {
		return
	}
	name := toks[2]
	if err := h.db.Remove(name, convID); err != nil {
		h.ChatDebug(convID, "handleRemove: failed to remove webhook: %s", err)
		return
	}
	h.ChatEcho(convID, "Success!")
}

func (h *Handler) handleList(cmd string, msg chat1.MsgSummary) {
	convID := msg.ConvID
	hooks, err := h.db.List(convID)
	if err != nil {
		h.ChatDebug(convID, "handleList: failed to list hook: %s", err)
		return
	}
	if !h.checkAdmin(msg) {
		return
	}

	if len(hooks) == 0 {
		h.ChatEcho(convID, "No hooks in this conversation")
		return
	}
	var body string
	for _, hook := range hooks {
		body += fmt.Sprintf("%s, %s\n", hook.name, h.formURL(hook.id))
	}
	h.ChatEcho(convID, body)
}

func (h *Handler) handleCreate(cmd string, msg chat1.MsgSummary) {
	convID := msg.ConvID
	toks := strings.Split(cmd, " ")
	if len(toks) != 3 {
		h.ChatDebug(convID, "invalid number of arguments, must specify a name")
		return
	}
	if !h.checkAdmin(msg) {
		return
	}

	name := toks[2]
	id, err := h.db.Create(name, convID)
	if err != nil {
		h.ChatDebug(convID, "handleCreate: failed to create webhook: %s", err)
		return
	}
	if _, err := h.kbc.SendMessageByTlfName(msg.Sender.Username, "%s", h.formURL(id)); err != nil {
		h.Debug(convID, "handleCreate: failed to send hook: %s", err)
	}
	h.ChatEcho(convID, "Success! New URL sent to @%s", msg.Sender.Username)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) {
	if msg.Content.Text == nil {
		h.Debug("skipping non-text message")
		return
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!webhook create"):
		h.handleCreate(cmd, msg)
	case strings.HasPrefix(cmd, "!webhook list"):
		h.handleList(cmd, msg)
	case strings.HasPrefix(cmd, "!webhook remove"):
		h.handleRemove(cmd, msg)
	default:
		h.Debug("ignoring unknown command")
	}
}
