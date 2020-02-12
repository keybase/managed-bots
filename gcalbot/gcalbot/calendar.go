package gcalbot

import (
	"context"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"google.golang.org/api/calendar/v3"
)

func (h *Handler) handleCalendarsList(msg chat1.MsgSummary, args []string) error {
	if len(args) != 1 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments.")
		return nil
	}

	keybaseUsername := msg.Sender.Username
	accountNickname := args[0]

	account, err := h.db.GetAccount(keybaseUsername, accountNickname)
	if err != nil {
		return err
	} else if account == nil {
		h.ChatEcho(msg.ConvID, "I couldn't find an account connection called '%s' :disappointed:", accountNickname)
		return nil
	}

	srv, err := GetCalendarService(account, h.oauth)
	if err != nil {
		return err
	}

	calendarList, err := getCalendarList(srv)
	if err != nil {
		return err
	}

	if len(calendarList) == 0 {
		h.ChatEcho(msg.ConvID,
			"There are no calendars associated with the account '%s'.", accountNickname)
		return nil
	}

	data := []interface{}{accountNickname}
	for _, calendarItem := range calendarList {
		if calendarItem.SummaryOverride != "" {
			data = append(data, calendarItem.SummaryOverride)
		} else {
			data = append(data, calendarItem.Summary)
		}
	}

	calendarListMessage := "Here are the calendars associated with the account '%s':" + strings.Repeat("\n• %s", len(calendarList))
	h.ChatEcho(msg.ConvID, calendarListMessage, data...)
	return nil
}

func getCalendarList(srv *calendar.Service) (list []*calendar.CalendarListEntry, err error) {
	err = srv.CalendarList.List().Pages(context.Background(), func(page *calendar.CalendarList) error {
		list = append(list, page.Items...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return list, nil
}
