package gcalbot

import (
	"context"
	"flag"
	"io/ioutil"
	"strings"

	"github.com/keybase/managed-bots/base"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type reminderFlags []int

func (r *reminderFlags) String() string {
	return ""
}

func (r *reminderFlags) Set(stringVal string) error {
	// TODO(marcel): improve this
	intVal, err := ParseMinutes(stringVal)
	if err != nil {
		return err
	}
	*r = append(*r, intVal)
	return nil
}

func (h *Handler) handleSubscribe(msg chat1.MsgSummary, args []string, unsubscribe bool) error {
	if len(args) < 1 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments.")
		return nil
	}

	keybaseUsername := msg.Sender.Username
	accountNickname := args[0]
	accountID := GetAccountID(keybaseUsername, accountNickname)

	var calendarName string
	var invites bool
	var reminders reminderFlags
	flags := flag.NewFlagSet("subscribe", flag.ContinueOnError)
	flags.SetOutput(ioutil.Discard)

	flags.StringVar(&calendarName, "cal", "", "calendar")
	flags.BoolVar(&invites, "invites", false, "invites")
	flags.Var(&reminders, "reminder", "reminder")

	err := flags.Parse(args[1:])
	if err != nil {
		h.ChatEcho(msg.ConvID, err.Error())
		return nil
	}

	client, err := base.GetOAuthClient(accountID, msg, h.kbc, h.config, h.db,
		h.getAccountOAuthOpts(msg, accountNickname))
	if err != nil || client == nil {
		// if no error, account doesn't exist, short circuit
		return err
	}

	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return err
	}

	var selectedCalendar *calendar.Calendar
	if calendarName == "" {
		// if no calendar is specified, default to the primary calendar
		selectedCalendar, err = getPrimaryCalendar(srv)
		if err != nil {
			return err
		}
	} else {
		calendarName = strings.ToLower(calendarName)
		err = srv.CalendarList.List().Pages(context.Background(), func(page *calendar.CalendarList) error {
			if selectedCalendar != nil {
				return nil
			}
			for _, item := range page.Items {
				if strings.ToLower(item.Summary) == calendarName {
					selectedCalendar, err = srv.Calendars.Get(item.Id).Do()
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	if selectedCalendar == nil {
		// TODO(marcel): add an error
		return nil
	}

	if !invites && len(reminders) == 0 {
		if unsubscribe {
			h.ChatEcho(msg.ConvID,
				"You must specify what notifications you wish to unsubscribe from using `--invite` or `--reminder minutesBefore`")
			return nil
		}
		// if no notifications are specified, default to a 10 minute reminder
		reminders = []int{10}
	}

	if invites {
		if !unsubscribe {
			err = h.subscribeInvites(srv, accountID, msg.ConvID, selectedCalendar)
		} else {
			err = h.unsubscribeInvites(srv, accountID, msg.ConvID, selectedCalendar)
		}
		if err != nil {
			return err
		}
	}

	for _, reminder := range reminders {
		if !unsubscribe {
			err = h.subscribeReminder(srv, accountID, msg.ConvID, selectedCalendar, reminder)
		} else {
			err = h.unsubscribeReminder(srv, accountID, msg.ConvID, selectedCalendar, reminder)
		}
		if err != nil {
			return err
		}
	}

	var action, preposition string
	if !unsubscribe {
		action = "subscribed"
		preposition = "to"
	} else {
		action = "unsubscribed"
		preposition = "from"
	}
	if invites && len(reminders) != 0 {
		h.ChatEcho(msg.ConvID,
			"OK, you have been %s %s invite notifications and %s %s event reminders for calendar %s [%s].",
			action, preposition, preposition, FormatMinuteSeriesString(reminders), selectedCalendar.Summary, accountNickname)
	} else if len(reminders) != 0 {
		h.ChatEcho(msg.ConvID,
			"OK, you have been %s %s %s event reminders for calendar %s [%s].",
			action, preposition, FormatMinuteSeriesString(reminders), selectedCalendar.Summary, accountNickname)
	} else {
		h.ChatEcho(msg.ConvID,
			"OK, you have been %s %s invite notifications for calendar %s [%s].",
			action, preposition, selectedCalendar.Summary, accountNickname)
	}

	return nil
}
