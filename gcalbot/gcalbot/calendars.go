package gcalbot

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

func (h *Handler) handleListCalendars(msg chat1.MsgSummary, args []string) error {
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
		return err
	} else if !exists {
		_, err = h.kbc.SendMessageByConvID(msg.ConvID,
			"No account connection with the nickname '%s' exists.", accountNickname)
		if err != nil {
			return fmt.Errorf("error sending message: %s", err)
		}
		return nil
	}

	identifier := fmt.Sprintf("%s:%s", username, accountNickname)
	client, err := base.GetOAuthClient(identifier, msg, h.kbc, h.requests, h.config, h.db, base.GetOAuthOpts{
		SkipAuthentication: true,
	})
	if err != nil {
		return err
	} else if client == nil {
		return fmt.Errorf("token was not found in storage for identifier '%s'", identifier)
	}

	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return err
	}

	calendarList, err := srv.CalendarList.List().Do()
	if err != nil {
		return err
	}

	if len(calendarList.Items) == 0 {
		_, err = h.kbc.SendMessageByConvID(msg.ConvID,
			"There are no calendars associated with the account '%s'.", accountNickname)
		if err != nil {
			return fmt.Errorf("error sending message: %s", err)
		}
		return nil
	}

	data := []interface{}{accountNickname}
	for _, calendarItem := range calendarList.Items {
		if calendarItem.SummaryOverride != "" {
			data = append(data, calendarItem.SummaryOverride)
		} else {
			data = append(data, calendarItem.Summary)
		}
	}

	calendarListMessage := "Here are the calendars associated with the account '%s':" + strings.Repeat("\n• %s", len(calendarList.Items))
	_, err = h.kbc.SendMessageByConvID(msg.ConvID, calendarListMessage, data...)
	if err != nil {
		return fmt.Errorf("error sending message: %s", err)
	}
	return nil
}
