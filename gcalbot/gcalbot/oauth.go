package gcalbot

import (
	"context"
	"fmt"

	"github.com/keybase/managed-bots/base"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"golang.org/x/oauth2"
)

// The browser side of connecting a Google account is a base.OAuthFlow served
// at oauthPath. The account is only connected once the requester sends
// "!gcal confirm <code>" from Keybase (handleConfirm).

const oauthPath = "/gcalbot/oauth"

const ConfirmCmd = "!gcal confirm"

const retryConnectCmd = "Run `!gcal accounts connect` again to get a new link."

func newOAuthFlow(debug *base.DebugOutput, oauthConfig *oauth2.Config, db *DB, botUsername string) *base.OAuthFlow {
	return base.NewOAuthFlow(debug, oauthConfig, oauthFlowStorage{DB: db}, ConfirmCmd, botUsername,
		"gcalbot", "/gcalbot/image/logo", oauthPath, oauth2.AccessTypeOffline)
}

func (h *Handler) requestOAuth(ctx context.Context, msg chat1.MsgSummary, accountNickname string) error {
	state, err := base.MakeRequestID()
	if err != nil {
		return err
	}

	err = h.db.PutState(ctx, state, OAuthRequest{
		OAuthRequest:    base.OAuthRequest{Requester: msg.Sender.Username, ConvID: msg.ConvID},
		AccountNickname: accountNickname,
	})
	if err != nil {
		return err
	}

	_, err = h.kbc.SendMessageByTlfName(msg.Sender.Username,
		"Visit %s to connect a Google account as '%s'.", base.OAuthStartURL(h.oauth, state), accountNickname)
	if err != nil {
		return err
	}

	// If we are in a 1-1 conv directly or as a bot user with the sender, skip this message.
	if !base.IsDirectPrivateMessage(h.kbc.GetUsername(), msg.Sender.Username, msg.Channel) {
		h.ChatEcho(msg.ConvID, "OK! I've sent a message to @%s to authorize me.", msg.Sender.Username)
	}

	return nil
}

// handleConfirm connects the Google account once the requester sends the code
// shown after authorizing.
func (h *Handler) handleConfirm(ctx context.Context, msg chat1.MsgSummary, code string) error {
	if code == "" {
		h.ChatEcho(msg.ConvID, "%s", base.ConfirmUsageMsg(ConfirmCmd))
		return nil
	}

	var account Account
	var req *OAuthRequest
	confirmed, err := base.ConfirmPendingOAuth(ctx, h.db, msg.Sender.Username, code,
		func(ctx context.Context, state string, token *oauth2.Token) (err error) {
			if req, err = h.db.GetState(ctx, state); err != nil {
				return err
			} else if req == nil {
				return fmt.Errorf("state expired before the account was saved")
			}
			account = Account{
				KeybaseUsername: req.Requester,
				AccountNickname: req.AccountNickname,
				Token:           *token,
			}
			return h.db.InsertAccount(ctx, account)
		})
	if err != nil {
		h.Errorf("handleConfirm: %v", err)
		h.ChatEcho(msg.ConvID, "%s", base.ConfirmSaveFailedMsg)
		return nil
	} else if !confirmed {
		h.ChatEcho(msg.ConvID, "%s", base.ConfirmNoMatchMsg(retryConnectCmd))
		return nil
	}

	// The account is connected; the default subscriptions are best effort.
	if err := h.createDefaultSubscriptions(ctx, &account, req.ConvID); err != nil {
		h.Errorf("handleConfirm: unable to create default subscriptions: %v", err)
		h.ChatEcho(msg.ConvID, "Connected a Google account as '%s', but I couldn't set up notifications. "+
			"Send `!gcal configure` in a conversation to set them up there.", req.AccountNickname)
		return nil
	}
	h.ChatEcho(msg.ConvID, "Connected a Google account as '%s'. Send `!gcal configure` in a conversation to set up notifications there.",
		req.AccountNickname)
	return nil
}

// createDefaultSubscriptions subscribes a 1on1 conv to invites and a 5 minute
// reminder for the account's primary calendar.
func (h *Handler) createDefaultSubscriptions(ctx context.Context, account *Account, convID chat1.ConvIDStr) error {
	conv, err := h.kbc.GetConversation(convID)
	if err != nil {
		return err
	}
	if !base.IsDirectPrivateMessage(h.kbc.GetUsername(), account.KeybaseUsername, conv.Channel) {
		return nil
	}

	srv, err := h.GetCalendarService(ctx, account)
	if err != nil {
		return err
	}
	primaryCalendar, err := srv.Calendars.Get("primary").Do()
	if err != nil {
		return err
	}

	if err := h.createSubscription(ctx, account, Subscription{
		CalendarID:    primaryCalendar.Id,
		KeybaseConvID: convID,
		Type:          SubscriptionTypeInvite,
	}); err != nil {
		return err
	}
	return h.createSubscription(ctx, account, Subscription{
		CalendarID:     primaryCalendar.Id,
		KeybaseConvID:  convID,
		DurationBefore: GetDurationFromMinutes(5),
		Type:           SubscriptionTypeReminder,
	})
}
