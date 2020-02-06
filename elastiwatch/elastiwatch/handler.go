package elastiwatch

import (
	"fmt"
	"strconv"
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
	logs    *LogWatch
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig,
	httpSrv *HTTPSrv, db *DB, logs *LogWatch) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		kbc:         kbc,
		httpSrv:     httpSrv,
		db:          db,
		logs:        logs,
	}
}

func (h *Handler) handleDefer(convID chat1.ConvIDStr, author, cmd string) error {
	toks := strings.Split(cmd, " ")
	if len(toks) < 3 {
		h.ChatEcho(convID, "must specify a regular expression")
		return nil
	}
	regex := strings.Join(toks[2:], " ")
	h.ChatEcho(convID, "adding deferral: %s", regex)
	if err := h.db.Create(regex, author); err != nil {
		return err
	}
	h.ChatEcho(convID, "Success!")
	return nil
}

func (h *Handler) handleDeferrals(convID chat1.ConvIDStr, cmd string) error {
	deferrals, err := h.db.List()
	if err != nil {
		return err
	}
	body := ""
	if len(deferrals) == 0 {
		h.ChatEcho(convID, "No deferrals in use")
		return nil
	}
	for _, d := range deferrals {
		body += fmt.Sprintf("id: %d author: %s regex: %s (created: %v)\n", d.id, d.author, d.regex, d.ctime)
	}
	h.ChatEcho(convID, body)
	return nil
}

func (h *Handler) handleUndefer(convID chat1.ConvIDStr, cmd string) error {
	toks := strings.Split(cmd, " ")
	if len(toks) < 3 {
		h.ChatEcho(convID, "must specify an ID")
		return nil
	}
	id, err := strconv.ParseInt(toks[2], 0, 0)
	if err != nil {
		h.ChatEcho(convID, "must specify a valid ID")
		return nil
	}
	h.ChatEcho(convID, "removing deferral: %d", id)
	if err := h.db.Remove(int(id)); err != nil {
		return err
	}
	h.ChatEcho(convID, "Success!")
	return nil
}

func (h *Handler) handleDump() error {
	h.logs.Peek()
	return nil
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		return nil
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!elastiwatch defer"):
		return h.handleDefer(msg.ConvID, msg.Sender.Username, cmd)
	case strings.HasPrefix(cmd, "!elastiwatch list-defers"):
		return h.handleDeferrals(msg.ConvID, cmd)
	case strings.HasPrefix(cmd, "!elastiwatch undefer"):
		return h.handleUndefer(msg.ConvID, cmd)
	case strings.HasPrefix(cmd, "!elastiwatch dump"):
		return h.handleDump()
	}
	return nil
}

func (h *Handler) HandleNewConv(chat1.ConvSummary) error {
	return nil
}
