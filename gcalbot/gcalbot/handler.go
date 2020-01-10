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

func (h *Handler) accountsListHandler(msg chat1.MsgSummary) error {
	username := msg.Sender.Username
	accounts, err := h.db.GetAccountsForUser(username)
	if err != nil {
		h.ChatDebug(msg.ConvID, "error fetching accounts from database %q", err)
		return nil
	}
	accountListMessage := "Here are your connected accounts:"
	for index := range accounts {
		accountListMessage += fmt.Sprintf("\n• %s", accounts[index])
	}
	_, err = h.kbc.SendMessageByConvID(msg.ConvID, accountListMessage)
	return err
}

func (h *Handler) accountsConnectHandler(cmd string, msg chat1.MsgSummary) error {
	toks, err := shellquote.Split(cmd)
	if err != nil {
		// TODO(marcel): possibly a better error message
		h.ChatDebug(msg.ConvID, "error splitting command: %s", err)
		return nil
	}
	args := toks[3:]
	if len(args) != 1 {
		// TODO(marcel): possibly a better error message
		h.ChatDebug(msg.ConvID, "bad args for accounts connect: %s", args)
		return nil
	}

	username := msg.Sender.Username
	accountNickname := args[0]

	exists, err := h.db.ExistsAccountForUser(username, accountNickname)
	if err != nil {
		h.ChatDebug(msg.ConvID, "error checking account: %s", err.Error())
		return nil
	}
	if exists {
		_, err = h.kbc.SendMessageByConvID(msg.ConvID,
			"An account connection with the nickname '%s' already exists.", accountNickname)
		return err
	}

	// TODO(marcel): is this the best method? adding more columns to the db complicates the existing oauth abstractions
	identifier := fmt.Sprintf("%s:%s", username, accountNickname)
	// TODO(marcel): this is very hacky
	authMessageTemplate := fmt.Sprintf("Visit %s to connect a Google account as '%s'.", "%s", accountNickname)

	_, err = base.GetOAuthClient(identifier, msg, h.kbc, h.requests, h.config, h.db,
		authMessageTemplate, base.GetOAuthOpts{OAuthOfflineAccessType: true})
	return err
}

func (h *Handler) accountsDisconnectHandler(cmd string, msg chat1.MsgSummary) error {
	toks, err := shellquote.Split(cmd)
	if err != nil {
		// TODO(marcel): possibly a better error message
		h.ChatDebug(msg.ConvID, "error splitting command: %s", err)
		return nil
	}
	args := toks[3:]
	if len(args) != 1 {
		// TODO(marcel): possibly a better error message
		h.ChatDebug(msg.ConvID, "bad args for accounts connect: %s", args)
		return nil
	}

	username := msg.Sender.Username
	accountNickname := args[0]

	exists, err := h.db.ExistsAccountForUser(username, accountNickname)
	if err != nil {
		h.ChatDebug(msg.ConvID, "error checking account: %s", err.Error())
		return nil
	}
	if !exists {
		_, err = h.kbc.SendMessageByConvID(msg.ConvID,
			"No account connection with the nickname '%s' exists.", accountNickname)
		return err
	}

	// TODO(marcel): is this the best method? adding more columns to the db complicates the existing oauth abstractions
	identifier := fmt.Sprintf("%s:%s", username, accountNickname)
	// TODO(marcel): wrap these into one transcation
	err = h.db.DeleteToken(identifier)
	if err != nil {
		return err
	}
	err = h.db.DeleteAccountForUser(username, accountNickname)
	if err != nil {
		return err
	}

	_, err = h.kbc.SendMessageByConvID(msg.ConvID, "Account '%s' has been disconnected.", accountNickname)
	return err
}
