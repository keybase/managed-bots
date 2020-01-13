package gcalbot

import (
	"fmt"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

func GetAccountIdentifier(username string, accountNickname string) string {
	return fmt.Sprintf("%s:%s", username, accountNickname)
}

func (h *Handler) HandleAuth(msg chat1.MsgSummary, identifier string) (err error) {
	defer func() {
		if err != nil {
			if _, err := h.kbc.SendMessageByConvID(msg.ConvID, "Something went wrong!"); err != nil {
				h.Debug("failed to send error message: %s", err)
			}
		}
	}()

	username := msg.Sender.Username
	if !strings.HasPrefix(identifier, username+":") {
		return fmt.Errorf("wrong identifier '%s' for username '%s'", identifier, username)
	}
	accountNickname := strings.TrimPrefix(identifier, username+":")

	err = h.db.InsertAccountForUser(username, accountNickname)
	if err != nil {
		return fmt.Errorf("error connecting account '%s': %s", accountNickname, err)
	}

	_, err = h.kbc.SendMessageByConvID(msg.ConvID, "Account '%s' has been connected.", accountNickname)
	if err != nil {
		h.Debug("error sending message: %s", err)
		return nil // no need to display an error on the web page, the account was successfully connected
	}

	cmd := strings.TrimSpace(msg.Content.Text.Body)
	if !strings.HasPrefix(cmd, "!gcal accounts connect") {
		if err = h.HandleCommand(msg); err != nil {
			h.ChatDebug(msg.ConvID, err.Error())
		}
	}
	return nil
}

func (h *Handler) accountsListHandler(msg chat1.MsgSummary) error {
	username := msg.Sender.Username
	accounts, err := h.db.GetAccountsForUser(username)
	if err != nil {
		return fmt.Errorf("error fetching accounts from database %q", err)
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

	identifier := GetAccountIdentifier(username, accountNickname)

	authURLCallback := func(authURL string) error {
		_, err = h.kbc.SendMessageByTlfName(msg.Sender.Username,
			"Visit %s to connect a Google account as '%s'.", authURL, accountNickname)
		return err
	}
	_, err = base.GetOAuthClient(identifier, msg, h.kbc, h.requests, h.config, h.db,
		base.GetOAuthOpts{
			AllowNonAdminForTeamAuth: true,
			OAuthOfflineAccessType:   true,
			AuthURLCallback:          authURLCallback,
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

func (h *Handler) getAccountOAuthOpts(msg chat1.MsgSummary, accountNickname string) base.GetOAuthOpts {
	return base.GetOAuthOpts{
		AllowNonAdminForTeamAuth: true,
		OAuthOfflineAccessType:   true,
		AuthURLCallback: func(authURL string) error {
			_, err := h.kbc.SendMessageByTlfName(msg.Sender.Username,
				"No account exists with the nickname '%s'. Visit %s to connect a Google account as '%s'.",
				accountNickname, authURL, accountNickname)
			return err
		},
	}
}
