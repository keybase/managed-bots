package gcalbot

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/googleapi"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func GetAccountID(keybaseUsername string, accountNickname string) string {
	return fmt.Sprintf("%s:%s", keybaseUsername, accountNickname)
}

func (h *Handler) HandleAuth(msg chat1.MsgSummary, accountID string) (err error) {
	defer func() {
		if err != nil {
			h.ChatEcho(msg.ConvID, "Something went wrong!")
		}
	}()

	keybaseUsername := msg.Sender.Username
	if !strings.HasPrefix(accountID, keybaseUsername+":") {
		return fmt.Errorf("wrong account ID '%s' for username '%s'", accountID, keybaseUsername)
	}
	accountNickname := strings.TrimPrefix(accountID, keybaseUsername+":")

	err = h.db.InsertAccount(Account{
		KeybaseUsername: keybaseUsername,
		AccountNickname: accountNickname,
		AccountID:       accountID,
	})
	if err != nil {
		return fmt.Errorf("error connecting account '%s': %s", accountNickname, err)
	}

	h.ChatEcho(msg.ConvID, "Account '%s' has been connected.", accountNickname)
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	if !strings.HasPrefix(cmd, "!gcal accounts connect") {
		if err = h.HandleCommand(msg); err != nil {
			h.ChatErrorf(msg.ConvID, err.Error())
		}
	}
	return nil
}

func (h *Handler) handleAccountsList(msg chat1.MsgSummary) error {
	username := msg.Sender.Username
	accounts, err := h.db.GetAccountNicknameListForUsername(username)
	if err != nil {
		return fmt.Errorf("error fetching accounts from database %q", err)
	}

	if accounts == nil {
		h.ChatEcho(msg.ConvID, "You have no connected accounts.")
		return nil
	}

	accountListMessage := "Here are your connected accounts:" + strings.Repeat("\n• %s", len(accounts))
	accountInterfaces := make([]interface{}, len(accounts))
	for index := range accounts {
		accountInterfaces[index] = accounts[index]
	}

	h.ChatEcho(msg.ConvID, accountListMessage, accountInterfaces...)
	return nil
}

func (h *Handler) handleAccountsConnect(msg chat1.MsgSummary, args []string) error {
	if len(args) != 1 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments.")
		return nil
	}

	keybaseUsername := msg.Sender.Username
	accountNickname := args[0]
	accountIdentifier := GetAccountID(keybaseUsername, accountNickname)

	exists, err := h.db.ExistsAccountForUsernameAndNickname(keybaseUsername, accountNickname)
	if err != nil {
		return fmt.Errorf("error checking for account: %s", err)
	} else if exists {
		// An account connection with the nickname already exists.
		return nil
	}

	authURLCallback := func(authURL string) error {
		_, err = h.kbc.SendMessageByTlfName(keybaseUsername,
			"Visit %s to connect a Google account as '%s'.", authURL, accountNickname)
		return err
	}
	_, err = base.GetOAuthClient(accountIdentifier, msg, h.kbc, h.requests, h.config, h.db,
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

func (h *Handler) handleAccountsDisconnect(msg chat1.MsgSummary, args []string) error {
	if len(args) != 1 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments.")
		return nil
	}

	keybaseUsername := msg.Sender.Username
	accountNickname := args[0]
	accountID := GetAccountID(keybaseUsername, accountNickname)

	exists, err := h.db.ExistsAccountForUsernameAndNickname(keybaseUsername, accountNickname)
	if err != nil {
		return fmt.Errorf("error checking for account: %s", err)
	} else if !exists {
		// No account connection with the nickname exists.
		return nil
	}

	err = h.deleteAccount(accountID)
	if err != nil {
		return err
	}

	h.ChatEcho(msg.ConvID, "Account '%s' has been disconnected.", accountNickname)
	return nil
}

func (h *Handler) deleteAccount(accountID string) error {
	token, err := h.db.GetToken(accountID)
	if err != nil || token == nil {
		return fmt.Errorf("error getting token: %s", err)
	}

	client := h.config.Client(context.Background(), token)
	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return err
	}

	channels, err := h.db.GetChannelListByAccountID(accountID)
	if err != nil {
		return err
	}

	for _, channel := range channels {
		err := srv.Channels.Stop(&calendar.Channel{
			Id:         channel.ChannelID,
			ResourceId: channel.ResourceID,
		}).Do()
		switch err := err.(type) {
		case nil:
		case *googleapi.Error:
			if err.Code == 404 {
				// if the channel wasn't found, continue
				continue
			}
			return err
		default:
			return err
		}
	}

	// cascading delete of account, oauth, subscriptions, channels and invites
	err = h.db.DeleteAccountByAccountID(accountID)

	return err
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
