package elastiwatch

import (
	"context"
	"fmt"
	"regexp"
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

func NewHandler(kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig, httpSrv *HTTPSrv, db *DB, logs *LogWatch) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		kbc:         kbc,
		httpSrv:     httpSrv,
		db:          db,
		logs:        logs,
	}
}

func (h *Handler) handleDefer(ctx context.Context, convID chat1.ConvIDStr, author, cmd string) error {
	toks := strings.Split(cmd, " ")
	if len(toks) < 3 {
		h.ChatEcho(convID, "must specify a regular expression")
		return nil
	}
	regex := strings.Join(toks[2:], " ")
	if _, err := regexp.Compile(regex); err != nil {
		h.ChatEcho(convID, "invalid regular expression: %s", err)
		return nil
	}
	if err := h.db.Create(ctx, regex, author); err != nil {
		return err
	}
	h.ChatEcho(convID, "Success!")
	return nil
}

func (h *Handler) handleDeferrals(ctx context.Context, convID chat1.ConvIDStr, _ string) error {
	deferrals, err := h.db.List(ctx)
	if err != nil {
		return err
	}
	var body strings.Builder
	if len(deferrals) == 0 {
		h.ChatEcho(convID, "No deferrals in use")
		return nil
	}
	for _, d := range deferrals {
		body.WriteString(fmt.Sprintf("id: %d author: %s regex: %s (created: %v)\n", d.ID, d.Author, d.Regex, d.Ctime))
	}
	h.ChatEcho(convID, "%s", body.String())
	return nil
}

func (h *Handler) handleUndefer(ctx context.Context, convID chat1.ConvIDStr, cmd string) error {
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
	if err := h.db.Remove(ctx, int(id)); err != nil {
		return err
	}
	h.ChatEcho(convID, "Success!")
	return nil
}

func (h *Handler) handleDump() error {
	h.logs.Peek()
	return nil
}

func (h *Handler) HandleCommand(ctx context.Context, msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		return nil
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!elastiwatch defer"):
		if ok, err := base.IsAtLeastWriter(h.kbc, msg.Sender.Username, msg.Channel); err != nil {
			return err
		} else if !ok {
			h.ChatEcho(msg.ConvID, "you must be at least a writer to use this command")
			return nil
		}
		return h.handleDefer(ctx, msg.ConvID, msg.Sender.Username, cmd)
	case strings.HasPrefix(cmd, "!elastiwatch list-defers"):
		return h.handleDeferrals(ctx, msg.ConvID, cmd)
	case strings.HasPrefix(cmd, "!elastiwatch undefer"):
		if ok, err := base.IsAtLeastWriter(h.kbc, msg.Sender.Username, msg.Channel); err != nil {
			return err
		} else if !ok {
			h.ChatEcho(msg.ConvID, "you must be at least a writer to use this command")
			return nil
		}
		return h.handleUndefer(ctx, msg.ConvID, cmd)
	case strings.HasPrefix(cmd, "!elastiwatch dump"):
		return h.handleDump()
	}
	return nil
}

func (h *Handler) HandleNewConv(context.Context, chat1.ConvSummary) error {
	return nil
}
