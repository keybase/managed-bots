package webhookbot

import (
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
)

type Handler struct {
	kbc        *kbchat.API
	db         *DB
	httpSrv    *HTTPSrv
	httpPrefix string
}

func NewHandler(kbc *kbchat.API, httpSrv *HTTPSrv, db *DB, httpPrefix string) *Handler {
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

func (h *Handler) chatEcho(convID, msg string, args ...interface{}) {
	if _, err := h.kbc.SendMessageByConvID(convID, msg, args); err != nil {
		h.debug("chatDebug: failed to send echo message: %s", err)
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

func (h *Handler) formURL(id string) string {
	return fmt.Sprintf("%s/%s", h.httpPrefix, id)
}

func (h *Handler) handleRemove(cmd, convID string) {

}

func (h *Handler) handleList(cmd, convID string) {
	hooks, err := h.db.List(convID)
	if err != nil {
		h.chatDebug(convID, "handleList: failed to list hook: %s", err)
		return
	}
	var body string
	for _, hook := range hooks {
		body += fmt.Sprintf("%s, %s\n", hook.name, h.formURL(hook.id))
	}
	h.chatEcho(convID, body)
}

func (h *Handler) handleCreate(cmd, convID string) {
	toks := strings.Split(cmd, " ")
	if len(toks) != 3 {
		h.chatDebug(convID, "invalid number of arguments, must specify a name")
		return
	}
	name := toks[2]
	id, err := h.db.Create(name, convID)
	if err != nil {
		h.chatDebug(convID, "handleCreate: failed to create webhook: %s", err)
		return
	}
	h.chatEcho(convID, "Success! %s", h.formURL(h.httpPrefix, id))
}

func (h *Handler) handleCommand(msg chat1.MsgSummary) {
	if msg.Content.Text == nil {
		h.debug("skipping non-text message")
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
		h.debug("ignoring unknown command")
	}
}
