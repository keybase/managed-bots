package gcalbot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"google.golang.org/api/googleapi"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func (h *Handler) handleAccountsList(ctx context.Context, msg chat1.MsgSummary) error {
	username := msg.Sender.Username
	accounts, err := h.db.GetAccountListForUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("error fetching accounts from database %q", err)
	}

	if accounts == nil {
		h.ChatEcho(msg.ConvID, "You have no connected accounts.")
		return nil
	}

	accountListMessage := "Here are your connected accounts:" + strings.Repeat("\n• %s", len(accounts))
	accountInterfaces := make([]any, len(accounts))
	for index := range accounts {
		accountInterfaces[index] = accounts[index].AccountNickname
	}

	h.ChatEcho(msg.ConvID, accountListMessage, accountInterfaces...)
	return nil
}

func (h *Handler) handleAccountsConnect(ctx context.Context, msg chat1.MsgSummary, args []string) error {
	if len(args) != 1 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments.")
		return nil
	}

	keybaseUsername := msg.Sender.Username
	accountNickname := args[0]

	exists, err := h.db.ExistsAccount(ctx, keybaseUsername, accountNickname)
	if err != nil {
		return fmt.Errorf("error checking for account: %s", err)
	} else if exists {
		// An account connection with the nickname already exists.
		return nil
	}

	return h.requestOAuth(ctx, msg, accountNickname)
}

func (h *Handler) handleAccountsDisconnect(ctx context.Context, msg chat1.MsgSummary, args []string) error {
	if len(args) != 1 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments.")
		return nil
	}

	keybaseUsername := msg.Sender.Username
	accountNickname := args[0]

	exists, err := h.db.ExistsAccount(ctx, keybaseUsername, accountNickname)
	if err != nil {
		return fmt.Errorf("error checking for account: %s", err)
	} else if !exists {
		// No account connection with the nickname exists.
		return nil
	}

	err = h.deleteAccount(ctx, keybaseUsername, accountNickname)
	if err != nil {
		return err
	}

	h.ChatEcho(msg.ConvID, "Account '%s' has been disconnected.", accountNickname)
	return nil
}

func (h *Handler) deleteAccount(ctx context.Context, keybaseUsername, accountNickname string) error {
	account, err := h.db.GetAccount(ctx, keybaseUsername, accountNickname)
	if err != nil || account == nil {
		return fmt.Errorf("error getting account: %s", err)
	}

	srv, err := h.GetCalendarServiceWithRetry(ctx, account)
	if err == nil {
		// Successfully got service, stop all channels before deleting
		channels, err := h.db.GetChannelListByAccount(ctx, account)
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
	} else if _, ok := err.(AccountAuthError); ok {
		// Auth already failed, can't stop channels but continue with deletion
		h.Debug("skipping channel cleanup for %s/%s due to auth failure", keybaseUsername, accountNickname)
	} else {
		// Unexpected error
		return err
	}

	// cascading delete of account, oauth, subscriptions, channels and invites
	err = h.db.DeleteAccount(ctx, keybaseUsername, accountNickname)

	return err
}

func GetCalendarService(ctx context.Context, account *Account, config *oauth2.Config, db *DB) (srv *calendar.Service, err error) {
	if account.Token.Expiry.Before(time.Now()) {
		newToken, err := config.TokenSource(ctx, &account.Token).Token()
		if err != nil {
			return nil, err
		}
		account.Token = *newToken
		err = db.InsertAccount(ctx, *account)
		if err != nil {
			return nil, fmt.Errorf("unable to update account token: %s", err)
		}
	}
	client := config.Client(ctx, &account.Token)
	return calendar.NewService(ctx, option.WithHTTPClient(client))
}

// GetCalendarServiceWithRetry wraps GetCalendarService and handles auth failures
// by deleting invalid credentials. Returns AccountAuthError if credentials were deleted.
func (h *Handler) GetCalendarServiceWithRetry(ctx context.Context, account *Account) (*calendar.Service, error) {
	srv, err := GetCalendarService(ctx, account, h.oauth, h.db)
	if err != nil && base.ShouldRetryAuth(err) {
		h.Errorf("auth failed for %s/%s, deleting credentials: %v", account.KeybaseUsername, account.AccountNickname, err)
		if delErr := h.db.DeleteAccount(ctx, account.KeybaseUsername, account.AccountNickname); delErr != nil {
			h.Errorf("failed to delete account after auth error: %v", delErr)
		}
		return nil, AccountAuthError{
			Username: account.KeybaseUsername,
			Nickname: account.AccountNickname,
		}
	}
	return srv, err
}

// handleAuthError sends a reconnection message to the user for AccountAuthError
func (h *Handler) handleAuthError(err error, accountNickname string, convID chat1.ConvIDStr) error {
	if _, ok := err.(AccountAuthError); !ok {
		return err
	}
	h.ChatEcho(convID, "Your account '%s' needs to be reconnected. Please run `!gcal accounts connect %s` again.", accountNickname, accountNickname)
	return nil
}

// handleAuthErrorDM sends a reconnection message via DM for AccountAuthError
func (h *Handler) handleAuthErrorDM(err error, account *Account) error {
	if _, ok := err.(AccountAuthError); !ok {
		return err
	}
	_, sendErr := h.kbc.SendMessageByTlfName(account.KeybaseUsername,
		"Your account '%s' needs to be reconnected. Please run `!gcal accounts connect %s` again.", account.AccountNickname, account.AccountNickname)
	return sendErr
}
