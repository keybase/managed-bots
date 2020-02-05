package gcalbot

import (
	"context"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func (h *Handler) subscribeReminder(
	srv *calendar.Service,
	accountID string,
	keybaseConvID chat1.ConvIDStr,
	selectedCalendar *calendar.Calendar,
	minutesBefore int,
) error {
	exists, err := h.createSubscription(srv, Subscription{
		AccountID:      accountID,
		CalendarID:     selectedCalendar.Id,
		KeybaseConvID:  keybaseConvID,
		DurationBefore: GetDurationFromMinutes(minutesBefore),
		Type:           SubscriptionTypeReminder,
	})
	if err != nil || exists {
		// if no error, subscription exists, short circuit
		return err
	}

	return nil
}

func (h *Handler) unsubscribeReminder(
	srv *calendar.Service,
	accountID string,
	keybaseConvID chat1.ConvIDStr,
	selectedCalendar *calendar.Calendar,
	minutesBefore int,
) error {
	exists, err := h.removeSubscription(srv, Subscription{
		AccountID:      accountID,
		CalendarID:     selectedCalendar.Id,
		KeybaseConvID:  keybaseConvID,
		DurationBefore: GetDurationFromMinutes(minutesBefore),
		Type:           SubscriptionTypeReminder,
	})
	if err != nil || !exists {
		// if no error, subscription doesn't exist, short circuit
		return err
	}

	// TODO(marcel): move message to calling function
	h.ChatEcho(keybaseConvID, "OK, you will no longer be reminded of events %s before they happen for your primary calendar '%s'.",
		FormatMinuteString(minutesBefore), selectedCalendar.Summary)

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

	minutesBeforeList, err := h.db.GetReminderDurationBeforeList(accountID, primaryCalendar.Id, msg.ConvID)
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
		data = append(data, FormatMinuteString(GetMinutesFromDuration(minutesBefore)))
	}

	calendarListMessage := "Here are the reminders associated with calendar '%s' for account '%s':" +
		strings.Repeat("\n• %s before an event", len(minutesBeforeList))
	h.ChatEcho(msg.ConvID, calendarListMessage, data...)

	return nil
}
