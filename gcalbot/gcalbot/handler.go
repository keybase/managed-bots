package gcalbot

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type Handler struct {
	*base.DebugOutput

	stats *base.StatsRegistry
	kbc   *kbchat.API
	db    *DB
	oauth *oauth2.Config

	reminderScheduler ReminderScheduler

	tokenSecret string

	httpPrefix string
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(
	stats *base.StatsRegistry,
	kbc *kbchat.API,
	debugConfig *base.ChatDebugOutputConfig,
	db *DB,
	oauth *oauth2.Config,
	reminderScheduler ReminderScheduler,
	tokenSecret string,
	httpPrefix string,
) *Handler {
	return &Handler{
		DebugOutput:       base.NewDebugOutput("Handler", debugConfig),
		stats:             stats.SetPrefix("Handler"),
		kbc:               kbc,
		db:                db,
		oauth:             oauth,
		reminderScheduler: reminderScheduler,
		tokenSecret:       tokenSecret,
		httpPrefix:        httpPrefix,
	}
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "Hello! I can get you set up with Google Calendar anytime, just send me `!gcal accounts connect <account nickname>`."
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
		h.ChatEcho(msg.ConvID, "%s", userErr)
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

	case strings.HasPrefix(cmd, "!gcal configure"):
		h.stats.Count("configure")
		return h.handleConfigure(msg)

	default:
		h.ChatEcho(msg.ConvID, "Unknown command %q", cmd)
		return nil
	}
}

func (h *Handler) handleReaction(msg chat1.MsgSummary) error {
	username := msg.Sender.Username
	messageID := msg.Content.Reaction.MessageID
	reaction := msg.Content.Reaction.Body

	invite, account, err := h.db.GetInviteAndAccountByUserMessage(username, messageID)
	if err != nil {
		return err
	} else if invite != nil && account != nil {
		err = h.updateEventResponseStatus(invite, account, InviteReaction(reaction))
		if err != nil {
			return fmt.Errorf("error updating event response status: %s", err)
		}
	}

	return nil
}

func (h *Handler) handleConfigure(msg chat1.MsgSummary) error {
	isAllowed, err := base.IsAtLeastWriter(h.kbc, msg.Sender.Username, msg.Channel)
	if err != nil {
		return err
	}
	if !isAllowed {
		h.ChatEcho(msg.ConvID,
			"Sorry, but you need to be at least a writer in order to configure Google Calendar notifications for this channel.")
		return nil
	}
	keybaseUsername := msg.Sender.Username
	token := h.LoginToken(keybaseUsername)

	query := url.Values{}
	query.Add("token", token)
	query.Add("username", keybaseUsername)
	query.Add("conv_id", string(msg.ConvID))
	link := fmt.Sprintf("%s/%s?%s", h.httpPrefix, "gcalbot", query.Encode())

	isPrivate := base.IsDirectPrivateMessage(h.kbc.GetUsername(), msg.Sender.Username, msg.Channel)

	body := fmt.Sprintf("Configure Google Calendar notifications %s: %s", GetConvHelpText(msg.Channel, isPrivate, true), link)
	if _, err := h.kbc.SendMessageByTlfName(keybaseUsername, "%s", body); err != nil {
		h.Debug("failed to send login attempt: %s", err)
	}

	// If we are in a 1-1 conv directly or as a bot user with the sender, skip this message.
	if !isPrivate {
		h.ChatEcho(msg.ConvID, "OK! I've sent a message to @%s to configure me.", msg.Sender.Username)
	}

	return nil
}

func (h *Handler) LoginToken(username string) string {
	return hex.EncodeToString(hmac.New(sha256.New, []byte(h.tokenSecret)).Sum([]byte(username)))
}
