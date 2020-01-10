package gcalbot

import (
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type Handler struct {
	*base.DebugOutput

	kbc      *kbchat.API
	db       *DB
	requests *base.OAuthRequests
	config   *oauth2.Config
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(kbc *kbchat.API, db *DB, requests *base.OAuthRequests, config *oauth2.Config) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", kbc),
		kbc:         kbc,
		db:          db,
		requests:    requests,
		config:      config,
	}
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "Hello! I can get you setup with a Google Meet video call anytime, just send me `!meet`."
	return base.HandleNewTeam(h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		h.Debug("skipping non-text message")
		return nil
	}

	cmd := strings.TrimSpace(msg.Content.Text.Body)

	if !strings.HasPrefix(cmd, "!gcal") {
		h.Debug("ignoring non-command message")
		return nil
	}

	switch {
	case cmd == "!gcal accounts":
		return h.accountsListHandler(msg)
	case strings.HasPrefix(cmd, "!gcal accounts connect"):
		return h.accountsConnectHandler(cmd, msg)
	case strings.HasPrefix(cmd, "!gcal accounts disconnect"):
		return h.accountsDisconnectHandler(cmd, msg)
	default:
		h.Debug("ignoring unknown command %q", cmd)
		// TODO(marcel): send user an error message
		return nil
	}
}
