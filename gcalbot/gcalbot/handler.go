package gcalbot

import (
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
	welcomeMsg := "Hello! I can get you setup with a Google Meet video call anytime, just send me `!meet`."
	return base.HandleNewTeam(h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleAuth(msg chat1.MsgSummary) error {
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	toks, err := shellquote.Split(cmd)
	if err != nil {
		h.ChatDebug(msg.ConvID, "error splitting command: %s", err)
		return nil
	}
	args := toks[3:]
	if len(args) != 1 {
		h.ChatDebug(msg.ConvID, "bad args for accounts connect: %s", args)
		return nil
	}

	username := msg.Sender.Username
	accountNickname := args[0]
	err = h.db.InsertAccountForUser(username, accountNickname)
	if err != nil {
		h.ChatDebug(msg.ConvID, "error connecting account %s", accountNickname)
		return err
	}

	_, err = h.kbc.SendMessageByConvID(msg.ConvID, "Account '%s' has been connected.", accountNickname)
	return err
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
