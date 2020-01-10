package gcalbot

import (
	"fmt"
	"strings"

	"github.com/kballard/go-shellquote"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

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

func (h *Handler) accountsListHandler(msg chat1.MsgSummary) error {
	username := msg.Sender.Username
	accounts, err := h.db.GetAccountsForUser(username)
	if err != nil {
		h.ChatDebug(msg.ConvID, "error fetching accounts from database %q", err)
		return nil
	}

	if len(accounts) == 0 {
		_, err = h.kbc.SendMessageByConvID(msg.ConvID, "You have no connected accounts.")
		if err != nil {
			return fmt.Errorf("error sending message: %s", err)
		}
		return nil
	}

	accountListMessage := "Here are your connected accounts:" + strings.Repeat("\n• %s", len(accounts))
	_, err = h.kbc.SendMessageByConvID(msg.ConvID, accountListMessage, accounts...)
	if err != nil {
		return fmt.Errorf("error sending message: %s", err)
	}
	return nil
}

func (h *Handler) accountsConnectHandler(msg chat1.MsgSummary, args []string) error {
	if len(args) != 1 {
		_, err := h.kbc.SendMessageByConvID(msg.ConvID, "Invalid number of arguments.")
		if err != nil {
			return fmt.Errorf("error sending message: %s", err)
		}
		return nil
	}

	username := msg.Sender.Username
	accountNickname := args[0]

	exists, err := h.db.ExistsAccountForUser(username, accountNickname)
	if err != nil {
		return fmt.Errorf("error checking for account: %s", err)
	}
	if exists {
		_, err = h.kbc.SendMessageByConvID(msg.ConvID,
			"An account connection with the nickname '%s' already exists.", accountNickname)
		if err != nil {
			return fmt.Errorf("error sending message: %s", err)
		}
		return nil
	}

	// TODO(marcel): is this the best method? adding more columns to the db complicates the existing oauth abstractions
	identifier := fmt.Sprintf("%s:%s", username, accountNickname)

	authURLCallback := func(authURL string) error {
		_, err = h.kbc.SendMessageByTlfName(msg.Sender.Username,
			"Visit %s to connect a Google account as '%s'.", authURL, accountNickname)
		return err
	}
	_, err = base.GetOAuthClient(identifier, msg, h.kbc, h.requests, h.config, h.db,
		base.GetOAuthOpts{
			OAuthOfflineAccessType: true,
			AuthURLCallback:        authURLCallback,
		})
	if err != nil {
		return fmt.Errorf("error authenticating user: %s", err)
	}
	return nil
}

func (h *Handler) accountsDisconnectHandler(msg chat1.MsgSummary, args []string) error {
	if len(args) != 1 {
		_, err := h.kbc.SendMessageByConvID(msg.ConvID, "Invalid number of arguments.")
		if err != nil {
			return fmt.Errorf("error sending message: %s", err)
		}
		return nil
	}

	username := msg.Sender.Username
	accountNickname := args[0]

	exists, err := h.db.ExistsAccountForUser(username, accountNickname)
	if err != nil {
		return fmt.Errorf("error checking for account: %s", err)
	}
	if !exists {
		_, err = h.kbc.SendMessageByConvID(msg.ConvID,
			"No account connection with the nickname '%s' exists.", accountNickname)
		if err != nil {
			return fmt.Errorf("error sending message: %s", err)
		}
		return nil
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
	if err != nil {
		return fmt.Errorf("error sending message: %s", err)
	}
	return nil
}
