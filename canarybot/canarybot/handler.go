package canarybot

import (
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type Handler struct {
	*base.DebugOutput

	stats *base.StatsRegistry
	kbc   *kbchat.API
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		stats:       stats.SetPrefix("Handler"),
		kbc:         kbc,
	}
}

func (h *Handler) handleEcho(cmd string, msg chat1.MsgSummary) error {
	body := strings.TrimPrefix(cmd, "!canary echo")
	if len(body) == 0 {
		h.ChatEcho(msg.ConvID, "uh-oh I need something to echo!")
	} else {
		h.ChatEcho(msg.ConvID, "%s", body)
	}
	return nil
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "Hey there I'm canarybot. Seems like I'm alive because you're getting this message. Happy days."
	return base.HandleNewTeam(h.stats, h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil || !strings.HasPrefix(msg.Content.Text.Body, "!canary") {
		return nil
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!canary echo"):
		return h.handleEcho(cmd, msg)
	default:
		h.ChatEcho(msg.ConvID, "Unknown command: %q", cmd)
	}
	return nil
}
