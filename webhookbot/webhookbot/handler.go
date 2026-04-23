package webhookbot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type Handler struct {
	*base.DebugOutput

	stats      *base.StatsRegistry
	kbc        *kbchat.API
	db         *DB
	httpSrv    *HTTPSrv
	httpPrefix string
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig, httpSrv *HTTPSrv, db *DB, httpPrefix string) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		stats:       stats.SetPrefix("Handler"),
		kbc:         kbc,
		db:          db,
		httpSrv:     httpSrv,
		httpPrefix:  httpPrefix,
	}
}

func (h *Handler) formURL(id string) string {
	return fmt.Sprintf("%s/webhookbot/%s", h.httpPrefix, id)
}

var errNotAllowed = errors.New("must be at least a writer to administer webhooks")

func (h *Handler) checkAllowed(msg chat1.MsgSummary) error {
	ok, err := base.IsAtLeastWriter(h.kbc, msg.Sender.Username, msg.Channel)
	if err != nil {
		return fmt.Errorf("handleCreate: failed to check role: %s", err)
	}
	if !ok {
		return errNotAllowed
	}
	return nil
}

func (h *Handler) handleRemove(ctx context.Context, cmd string, msg chat1.MsgSummary) (err error) {
	convID := msg.ConvID
	toks := strings.Split(cmd, " ")
	if len(toks) != 3 {
		h.ChatEcho(convID, "invalid number of arguments, must specify a name")
		return nil
	}
	err = h.checkAllowed(msg)
	switch err {
	case nil:
	case errNotAllowed:
		h.ChatEcho(convID, "%s", err.Error())
		return nil
	default:
		return err
	}
	h.stats.Count("remove")
	name := toks[2]
	if err := h.db.Remove(ctx, name, convID); err != nil {
		return fmt.Errorf("handleRemove: failed to remove webhook: %s", err)
	}
	h.ChatEcho(convID, "Success!")
	return nil
}

func (h *Handler) handleList(ctx context.Context, _ string, msg chat1.MsgSummary) (err error) {
	convID := msg.ConvID
	hooks, err := h.db.List(ctx, convID)
	if err != nil {
		return fmt.Errorf("handleList: failed to list hook: %s", err)
	}
	err = h.checkAllowed(msg)
	switch err {
	case nil:
	case errNotAllowed:
		h.ChatEcho(convID, "%s", err.Error())
		return nil
	default:
		return err
	}

	h.stats.Count("list")
	if len(hooks) == 0 {
		h.ChatEcho(convID, "No hooks in this conversation")
		return nil
	}
	var body strings.Builder
	for _, hook := range hooks {
		body.WriteString(fmt.Sprintf("%s, %s\n", hook.Name, h.formURL(hook.ID)))
	}
	if _, err := h.kbc.SendMessageByTlfName(msg.Sender.Username, "%s", body.String()); err != nil {
		h.Debug("handleList: failed to send hook: %s", err)
	}
	h.ChatEcho(convID, "List sent to @%s", msg.Sender.Username)
	return nil
}

func (h *Handler) handleCreate(ctx context.Context, cmd string, msg chat1.MsgSummary) (err error) {
	convID := msg.ConvID
	toks := strings.Split(cmd, " ")
	if len(toks) != 3 {
		h.ChatEcho(convID, "invalid number of arguments, must specify a name")
		return nil
	}
	err = h.checkAllowed(msg)
	switch err {
	case nil:
	case errNotAllowed:
		h.ChatEcho(convID, "%s", err.Error())
		return nil
	default:
		return err
	}

	h.stats.Count("create")
	name := toks[2]
	id, err := h.db.Create(ctx, name, convID)
	if err != nil {
		return fmt.Errorf("handleCreate: failed to create webhook: %s", err)
	}
	if _, err := h.kbc.SendMessageByTlfName(msg.Sender.Username, "%s", h.formURL(id)); err != nil {
		h.Debug("handleCreate: failed to send hook: %s", err)
	}
	h.ChatEcho(convID, "Success! New URL sent to @%s", msg.Sender.Username)
	return nil
}

func (h *Handler) HandleNewConv(_ context.Context, conv chat1.ConvSummary) error {
	welcomeMsg := "I can create generic webhooks into Keybase! Try `!webhook create` to get started."
	return base.HandleNewTeam(h.stats, h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleCommand(ctx context.Context, msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		return nil
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!webhook create"):
		return h.handleCreate(ctx, cmd, msg)
	case strings.HasPrefix(cmd, "!webhook list"):
		return h.handleList(ctx, cmd, msg)
	case strings.HasPrefix(cmd, "!webhook remove"):
		return h.handleRemove(ctx, cmd, msg)
	}
	return nil
}
