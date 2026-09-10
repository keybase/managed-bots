package gcalbot

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/oauth2"

	"google.golang.org/api/googleapi"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
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

	srv, err := getCalendarService(ctx, account, h.oauth, h.db)
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
	} else if base.ShouldRetryAuth(err) {
		// Auth already failed, can't stop channels but continue with deletion
		h.Debug("skipping channel cleanup for %s/%s due to auth failure", keybaseUsername, accountNickname)
	} else {
		return err
	}

	// cascading delete of account, oauth, subscriptions, channels and invites
	return h.db.DeleteAccount(ctx, keybaseUsername, accountNickname)
}

func getCalendarService(ctx context.Context, account *Account, config *oauth2.Config, db *DB) (*calendar.Service, error) {
	src := base.PersistTokenSource(ctx, &account.Token, config.TokenSource(ctx, &account.Token),
		func(ctx context.Context, _ *oauth2.Token) error {
			return db.InsertAccount(ctx, *account)
		})
	if _, err := src.Token(); err != nil {
		return nil, err
	}
	return calendar.NewService(ctx, option.WithHTTPClient(oauth2.NewClient(ctx, src)))
}

const reconnectAccountMsg = "Your account '%s' needs to be reconnected. Please run `!gcal accounts connect %s` again."

// CalendarAuth obtains a Calendar client and recovers from invalid OAuth credentials.
type CalendarAuth struct {
	oauth *oauth2.Config
	db    *DB
	debug *base.DebugOutput
	kbc   *kbchat.API
}

func NewCalendarAuth(oauth *oauth2.Config, db *DB, debug *base.DebugOutput, kbc *kbchat.API) *CalendarAuth {
	return &CalendarAuth{oauth: oauth, db: db, debug: debug, kbc: kbc}
}

func (c *CalendarAuth) GetCalendarService(ctx context.Context, account *Account) (*calendar.Service, error) {
	srv, err := getCalendarService(ctx, account, c.oauth, c.db)
	if err != nil {
		return nil, c.InvalidateIfAuthError(ctx, account, err)
	}
	return srv, nil
}

// InvalidateIfAuthError deletes the account and DMs the user when err
// indicates invalid OAuth credentials. Returns AccountAuthError in that case,
// otherwise returns err unchanged.
func (c *CalendarAuth) InvalidateIfAuthError(ctx context.Context, account *Account, err error) error {
	if err == nil || !base.ShouldRetryAuth(err) {
		return err
	}
	c.debug.Errorf("auth failed for %s/%s, deleting credentials: %v", account.KeybaseUsername, account.AccountNickname, err)
	if delErr := c.db.DeleteAccount(ctx, account.KeybaseUsername, account.AccountNickname); delErr != nil {
		c.debug.Errorf("failed to delete account after auth error: %v", delErr)
	}
	if _, sendErr := c.kbc.SendMessageByTlfName(account.KeybaseUsername, reconnectAccountMsg,
		account.AccountNickname, account.AccountNickname); sendErr != nil {
		c.debug.Errorf("failed to DM user after auth error: %v", sendErr)
	}
	return AccountAuthError{
		Username: account.KeybaseUsername,
		Nickname: account.AccountNickname,
	}
}

// WrapAuth invalidates the account on auth failure and returns nil in that
// case so callers can treat reconnect-needed as handled.
func (c *CalendarAuth) WrapAuth(ctx context.Context, account *Account, err error) error {
	return IgnoreAccountAuthError(c.InvalidateIfAuthError(ctx, account, err))
}
