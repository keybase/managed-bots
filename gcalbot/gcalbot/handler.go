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

	stats  *base.StatsRegistry
	kbc    *kbchat.API
	db     *DB
	config *oauth2.Config

	reminderScheduler ReminderScheduler

	httpPrefix string
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(
	stats *base.StatsRegistry,
	kbc *kbchat.API,
	debugConfig *base.ChatDebugOutputConfig,
	db *DB,
	config *oauth2.Config,
	reminderScheduler ReminderScheduler,
	httpPrefix string,
) *Handler {
	return &Handler{
		DebugOutput:       base.NewDebugOutput("Handler", debugConfig),
		stats:             stats.SetPrefix("Handler"),
		kbc:               kbc,
		db:                db,
		config:            config,
		reminderScheduler: reminderScheduler,
		httpPrefix:        httpPrefix,
	}
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "Hello! I can get you setup with Google Calendar anytime, just send me `!gcal accounts connect <account nickname>`."
	return base.HandleNewTeam(h.stats, h.DebugOutput, h.kbc, conv, welcomeMsg)
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
		h.stats.Count("accounts list")
		return h.handleAccountsList(msg)
	case strings.HasPrefix(cmd, "!gcal accounts connect"):
		h.stats.Count("accounts connect")
		return h.handleAccountsConnect(msg, tokens[3:])
	case strings.HasPrefix(cmd, "!gcal accounts disconnect"):
		h.stats.Count("accounts disconnect")
		return h.handleAccountsDisconnect(msg, tokens[3:])
	case strings.HasPrefix(cmd, "!gcal calendars list"):
		h.stats.Count("calendars list")
		return h.handleCalendarsList(msg, tokens[3:])
	case strings.HasPrefix(cmd, "!gcal invites subscribe"):
		h.stats.Count("invites subscribe")
		return h.handleInvitesSubscribe(msg, tokens[3:])
	case strings.HasPrefix(cmd, "!gcal invites unsubscribe"):
		h.stats.Count("invites unsubscribe")
		return h.handleInvitesUnsubscribe(msg, tokens[3:])
	case strings.HasPrefix(cmd, "!gcal reminders subscribe"):
		h.stats.Count("reminders subscribe")
		return h.handleRemindersSubscribe(msg, tokens[3:])
	case strings.HasPrefix(cmd, "!gcal reminders unsubscribe"):
		h.stats.Count("reminders unsubscribe")
		return h.handleRemindersUnsubscribe(msg, tokens[3:])
	case strings.HasPrefix(cmd, "!gcal reminders list"):
		h.stats.Count("reminders list")
		return h.handleRemindersList(msg, tokens[3:])
	default:
		h.ChatEcho(msg.ConvID, "Unknown command %q", cmd)
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
