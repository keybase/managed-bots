package gcalbot

import (
	"context"

	"github.com/keybase/managed-bots/base"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

func (h *Handler) handleRemindersSubscribe(msg chat1.MsgSummary, args []string) error {
	if len(args) != 3 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments.")
		return nil
	}

	keybaseUsername := msg.Sender.Username
	accountNickname := args[0]
	accountID := GetAccountID(keybaseUsername, accountNickname)

	client, err := base.GetOAuthClient(accountID, msg, h.kbc, h.requests, h.config, h.db,
		h.getAccountOAuthOpts(msg, accountNickname))
	if err != nil || client == nil {
		// if no error, account doesn't exist, short circuit
		return err
	}

	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return err
	}

	duration, userErr, err := ParseReminderDuration(args[1], args[2])
	if err != nil {
		return err
	} else if userErr != "" {
		h.ChatEcho(msg.ConvID, userErr)
		return nil
	}

	primaryCalendar, err := getPrimaryCalendar(srv)
	if err != nil {
		return err
	}

	h.ChatEcho(msg.ConvID, "OK, you will be reminded of events %s before they happen for your primary calendar '%s'.",
		duration, primaryCalendar.Id)

	return nil
}

func (h *Handler) handleRemindersUnsubscribe(msg chat1.MsgSummary, args []string) error {
	if len(args) != 3 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments.")
		return nil
	}

	keybaseUsername := msg.Sender.Username
	accountNickname := args[0]
	accountID := GetAccountID(keybaseUsername, accountNickname)

	token, err := h.db.GetToken(accountID)
	if err != nil || token == nil {
		// if no error, account doesn't exist, short circuit
		return err
	}

	client := h.config.Client(context.Background(), token)
	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return err
	}

	duration, userErr, err := ParseReminderDuration(args[1], args[2])
	if err != nil {
		return err
	} else if userErr != "" {
		h.ChatEcho(msg.ConvID, userErr)
		return nil
	}

	primaryCalendar, err := getPrimaryCalendar(srv)
	if err != nil {
		return err
	}

	h.ChatEcho(msg.ConvID, "OK, you will no longer be reminded of events %s before they happen for your primary calendar '%s'.", duration, primaryCalendar.Id)

	return nil
}

func (h *Handler) handleRemindersList(msg chat1.MsgSummary, args []string) error {
	if len(args) != 1 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments.")
		return nil
	}

	// keybaseUsername := msg.Sender.Username
	accountNickname := args[0]
	// accountID := GetAccountID(keybaseUsername, accountNickname)

	h.ChatEcho(msg.ConvID, "There are no calendars associated with the account '%s'.", accountNickname)

	return nil
}
