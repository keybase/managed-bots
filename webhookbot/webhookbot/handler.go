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

func NewHandler(kbc *kbchat.API, httpSrv *HTTPSrv, db *DB, httpPrefix string) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", kbc),
		kbc:         kbc,
		db:          db,
		httpSrv:     httpSrv,
		httpPrefix:  httpPrefix,
	}
}

func (h *Handler) Listen() error {
	sub, err := h.kbc.ListenForNewTextMessages()
	if err != nil {
		h.Debug("Listen: failed to listen: %s", err)
		return err
	}
	h.Debug("startup success, listening for messages...")
	for {
		msg, err := sub.Read()
		if err != nil {
			h.Debug("Listen: Read() error: %s", err)
			continue
		}
		h.handleCommand(msg.Message)
	}
}

func (h *Handler) formURL(id string) string {
	return fmt.Sprintf("%s/%s", h.httpPrefix, id)
}

func (h *Handler) handleRemove(cmd, convID string) {
	toks := strings.Split(cmd, " ")
	if len(toks) != 3 {
		h.ChatDebug(convID, "invalid number of arguments, must specify a name")
		return
	}
	name := toks[2]
	if err := h.db.Remove(name, convID); err != nil {
		h.ChatDebug(convID, "handleRemove: failed to remove webhook: %s", err)
		return
	}
	h.ChatEcho(convID, "Success!")
}

func (h *Handler) handleList(cmd, convID string) {
	hooks, err := h.db.List(convID)
	if err != nil {
		h.ChatDebug(convID, "handleList: failed to list hook: %s", err)
		return
	}
	var body string
	for _, hook := range hooks {
		body += fmt.Sprintf("%s, %s\n", hook.name, h.formURL(hook.id))
	}
	h.ChatEcho(convID, body)
}

func (h *Handler) handleCreate(cmd, convID string) {
	toks := strings.Split(cmd, " ")
	if len(toks) != 3 {
		h.ChatDebug(convID, "invalid number of arguments, must specify a name")
		return
	}
	name := toks[2]
	id, err := h.db.Create(name, convID)
	if err != nil {
		h.ChatDebug(convID, "handleCreate: failed to create webhook: %s", err)
		return
	}
	h.ChatEcho(convID, "Success! %s", h.formURL(id))
}

func (h *Handler) handleCommand(msg chat1.MsgSummary) {
	if msg.Content.Text == nil {
		h.Debug("skipping non-text message")
		return
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!webhook create"):
		h.handleCreate(cmd, msg.ConvID)
	case strings.HasPrefix(cmd, "!webhook list"):
		h.handleList(cmd, msg.ConvID)
	case strings.HasPrefix(cmd, "!webhook remove"):
		h.handleRemove(cmd, msg.ConvID)
	default:
		h.Debug("ignoring unknown command")
	}
}
