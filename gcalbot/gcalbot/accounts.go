package gcalbot

import (
	"fmt"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

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

	accountListMessage := "Here are your connected accounts:"
	for index := range accounts {
		accountListMessage += fmt.Sprintf("\n• %s", accounts[index])
	}
	_, err = h.kbc.SendMessageByConvID(msg.ConvID, accountListMessage)
	if err != nil {
		return fmt.Errorf("error sending message: %s", err)
	}
	return nil
}

func (h *Handler) accountsConnectHandler(cmd string, msg chat1.MsgSummary) error {
	args, err := base.ArgumentsFromCmd(cmd, 3)
	if len(args) != 1 {
		_, err = h.kbc.SendMessageByConvID(msg.ConvID, "Invalid number of arguments.")
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
	// TODO(marcel): this is very hacky
	authMessageTemplate := fmt.Sprintf("Visit %s to connect a Google account as '%s'.", "%s", accountNickname)

	_, err = base.GetOAuthClient(identifier, msg, h.kbc, h.requests, h.config, h.db,
		authMessageTemplate, base.GetOAuthOpts{OAuthOfflineAccessType: true})
	if err != nil {
		return fmt.Errorf("error authenticating user: %s", err)
	}
	return nil
}

func (h *Handler) accountsDisconnectHandler(cmd string, msg chat1.MsgSummary) error {
	args, err := base.ArgumentsFromCmd(cmd, 3)
	if len(args) != 1 {
		_, err = h.kbc.SendMessageByConvID(msg.ConvID, "Invalid number of arguments.")
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
