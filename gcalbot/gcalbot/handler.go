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

	kbc        *kbchat.API
	db         *DB
	requests   *base.OAuthRequests
	config     *oauth2.Config
	httpPrefix string
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(
	kbc *kbchat.API,
	debugConfig *base.ChatDebugOutputConfig,
	db *DB,
	requests *base.OAuthRequests,
	config *oauth2.Config,
	httpPrefix string,
) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		kbc:         kbc,
		db:          db,
		requests:    requests,
		config:      config,
		httpPrefix:  httpPrefix,
	}
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "Hello! I can get you setup with Google Calendar anytime, just send me `!gcal accounts connect <account nickname>`."
	return base.HandleNewTeam(h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Reaction != nil && msg.Sender.Username != h.kbc.GetUsername() {
		return h.handleReaction(msg)
	}

	if msg.Content.Text == nil {
		return nil
	}

	cmd := strings.TrimSpace(msg.Content.Text.Body)

	if !strings.HasPrefix(cmd, "!gcal") {
		return nil
	}

	tokens, userErr, err := base.SplitTokens(cmd)
	if err != nil {
		return err
	} else if userErr != "" {
		h.ChatEcho(msg.ConvID, userErr)
		return nil
	}

	switch {
	case strings.HasPrefix(cmd, "!gcal accounts list"):
		return h.accountsListHandler(msg)
	case strings.HasPrefix(cmd, "!gcal accounts connect"):
		return h.accountsConnectHandler(msg, tokens[3:])
	case strings.HasPrefix(cmd, "!gcal accounts disconnect"):
		return h.accountsDisconnectHandler(msg, tokens[3:])
	case strings.HasPrefix(cmd, "!gcal list-calendars"):
		return h.handleListCalendars(msg, tokens[2:])
	case strings.HasPrefix(cmd, "!gcal subscribe invites"):
		return h.handleSubscribeInvites(msg, tokens[3:])
	case strings.HasPrefix(cmd, "!gcal unsubscribe invites"):
		return h.handleUnsubscribeInvites(msg, tokens[3:])
	default:
		h.ChatEcho(msg.ConvID, "Unknown command.")
		return nil
	}
}

func (h *Handler) handleReaction(msg chat1.MsgSummary) error {
	username := msg.Sender.Username
	messageID := uint(msg.Content.Reaction.MessageID)
	reaction := msg.Content.Reaction.Body

	invite, err := h.db.GetInviteEventByUserMessage(username, messageID)
	if err != nil {
		return err
	} else if invite != nil {
		return h.updateEventResponseStatus(invite, InviteReaction(reaction))
	}

	return nil
}
