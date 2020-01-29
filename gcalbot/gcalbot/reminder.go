package gcalbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func (h *Handler) handleRemindersSubscribe(msg chat1.MsgSummary, args []string) error {
	if len(args) != 2 {
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

	minutesBefore, userErr, err := parseMinutes(args[1])
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

	exists, err := h.createSubscription(srv, Subscription{
		AccountID:     accountID,
		CalendarID:    primaryCalendar.Id,
		KeybaseConvID: msg.ConvID,
		MinutesBefore: minutesBefore,
		Type:          SubscriptionTypeReminder,
	})
	if err != nil || exists {
		// if no error, subscription exists, short circuit
		return err
	}

	h.ChatEcho(msg.ConvID, "OK, you will be reminded of events %s before they happen for your primary calendar '%s'.",
		MinutesBeforeString(minutesBefore), primaryCalendar.Id)

	return nil
}

func (h *Handler) handleRemindersUnsubscribe(msg chat1.MsgSummary, args []string) error {
	if len(args) != 2 {
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

	minutesBefore, userErr, err := parseMinutes(args[1])
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

	exists, err := h.removeSubscription(srv, Subscription{
		AccountID:     accountID,
		CalendarID:    primaryCalendar.Id,
		KeybaseConvID: msg.ConvID,
		MinutesBefore: minutesBefore,
		Type:          SubscriptionTypeReminder,
	})
	if err != nil || !exists {
		// if no error, subscription doesn't exist, short circuit
		return err
	}

	h.ChatEcho(msg.ConvID, "OK, you will no longer be reminded of events %s before they happen for your primary calendar '%s'.",
		MinutesBeforeString(minutesBefore), primaryCalendar.Id)

	return nil
}

func (h *Handler) handleRemindersList(msg chat1.MsgSummary, args []string) error {
	if len(args) != 1 {
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

	primaryCalendar, err := getPrimaryCalendar(srv)
	if err != nil {
		return err
	}

	minutesBeforeList, err := h.db.GetReminderMinutesBeforeList(accountID, primaryCalendar.Id, msg.ConvID)
	if err != nil {
		return err
	}

	if len(minutesBeforeList) == 0 {
		h.ChatEcho(msg.ConvID, "There are no reminders associated with calendar '%s' for account '%s'.",
			primaryCalendar.Summary, accountNickname)
		return nil
	}

	data := []interface{}{primaryCalendar.Summary, accountNickname}
	for _, minutesBefore := range minutesBeforeList {
		data = append(data, MinutesBeforeString(minutesBefore))
	}

	calendarListMessage := "Here are the reminders associated with calendar '%s' for account '%s':" +
		strings.Repeat("\n• %s before an event", len(minutesBeforeList))
	h.ChatEcho(msg.ConvID, calendarListMessage, data...)

	return nil
}

func parseMinutes(arg string) (minutes int, userErrorMessage string, err error) {
	minutesBefore, err := strconv.Atoi(arg)
	switch err := err.(type) {
	case nil:
	case *strconv.NumError:
		if err.Err == strconv.ErrSyntax || err.Err == strconv.ErrRange {
			userErrorMessage = fmt.Sprintf("Error parsing minutes before start of event: %s", err.Err.Error())
			return 0, userErrorMessage, nil
		} else {
			return 0, "", err
		}
	default:
		return 0, "", err
	}
	if minutesBefore < 0 {
		userErrorMessage = "Minutes before start of event cannot be negative."
		return 0, userErrorMessage, nil
	} else if minutesBefore > 60 {
		userErrorMessage = "Minutes before start of event cannot be greater than 60."
		return 0, userErrorMessage, nil
	}
	return minutesBefore, "", nil
}

func MinutesBeforeString(minutesBefore int) string {
	if minutesBefore == 1 {
		return "1 minute"
	} else {
		return fmt.Sprintf("%d minutes", minutesBefore)
	}
}
