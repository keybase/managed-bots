package gcalbot

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/oauth2"

	"google.golang.org/api/googleapi"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func (h *Handler) handleAccountsList(msg chat1.MsgSummary) error {
	username := msg.Sender.Username
	accounts, err := h.db.GetAccountListForUsername(username)
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
		accountInterfaces[index] = accounts[index].AccountNickname
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

	exists, err := h.db.ExistsAccount(keybaseUsername, accountNickname)
	if err != nil {
		return fmt.Errorf("error checking for account: %s", err)
	} else if exists {
		// An account connection with the nickname already exists.
		return nil
	}

	return h.requestOAuth(msg, accountNickname)
}

func (h *Handler) handleAccountsDisconnect(msg chat1.MsgSummary, args []string) error {
	if len(args) != 1 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments.")
		return nil
	}

	keybaseUsername := msg.Sender.Username
	accountNickname := args[0]

	exists, err := h.db.ExistsAccount(keybaseUsername, accountNickname)
	if err != nil {
		return fmt.Errorf("error checking for account: %s", err)
	} else if !exists {
		// No account connection with the nickname exists.
		return nil
	}

	err = h.deleteAccount(keybaseUsername, accountNickname)
	if err != nil {
		return err
	}

	h.ChatEcho(msg.ConvID, "Account '%s' has been disconnected.", accountNickname)
	return nil
}

func (h *Handler) deleteAccount(keybaseUsername, accountNickname string) error {
	account, err := h.db.GetAccount(keybaseUsername, accountNickname)
	if err != nil || account == nil {
		return fmt.Errorf("error getting account: %s", err)
	}

	srv, err := GetCalendarService(account, h.oauth)
	if err != nil {
		return err
	}

	channels, err := h.db.GetChannelListByAccount(account)
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
	err = h.db.DeleteAccount(keybaseUsername, accountNickname)

	return err
}

func GetCalendarService(account *Account, config *oauth2.Config) (srv *calendar.Service, err error) {
	client := config.Client(context.Background(), &account.Token)
	return calendar.NewService(context.Background(), option.WithHTTPClient(client))
}
