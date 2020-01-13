package gcalbot

import (
	"fmt"
	"strings"

	"github.com/kballard/go-shellquote"

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
	welcomeMsg := "Hello! I can get you setup with Google Calendar anytime, just send me `!gcal accounts connect <account nickname>`."
	return base.HandleNewTeam(h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		h.Debug("skipping non-text message")
		return nil
	}

	cmd := strings.TrimSpace(msg.Content.Text.Body)

	if !strings.HasPrefix(cmd, "!gcal") {
		return nil
	}

	tokens, err := shellquote.Split(cmd)
	if err != nil {
		// TODO(marcel): send better error message to user
		return fmt.Errorf("error splitting command string: %s", err)
	}

	switch {
	case strings.HasPrefix(cmd, "!gcal accounts"):
		if !(msg.Sender.Username == msg.Channel.Name) {
			_, err = h.kbc.SendMessageByConvID(msg.ConvID, "All `!gcal accounts` commands must be sent over direct message.")
			if err != nil {
				return fmt.Errorf("error sending message: %s", err)
			}
			_, err = h.kbc.SendMessageByTlfName(msg.Sender.Username, "Feel free to send me `!gcal accounts` commands here, over direct message.")
			if err != nil {
				return fmt.Errorf("error sending message: %s", err)
			}
			return nil
		}
		switch {
		case strings.HasPrefix(cmd, "!gcal accounts list"):
			return h.accountsListHandler(msg)
		case strings.HasPrefix(cmd, "!gcal accounts connect"):
			return h.accountsConnectHandler(msg, tokens[3:])
		case strings.HasPrefix(cmd, "!gcal accounts disconnect"):
			return h.accountsDisconnectHandler(msg, tokens[3:])
		}
		fallthrough
	default:
		_, err = h.kbc.SendMessageByConvID(msg.ConvID, "Unknown command.")
		if err != nil {
			h.Debug("error sending message: %s", err)
		}
		return nil
	}
}
