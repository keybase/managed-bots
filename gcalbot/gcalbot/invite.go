package gcalbot

import (
	"context"
	"fmt"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"google.golang.org/api/calendar/v3"
)

type InviteReaction string

const (
	InviteReactionYes   InviteReaction = "Yes 👍"
	InviteReactionNo    InviteReaction = "No 👎"
	InviteReactionMaybe InviteReaction = "Maybe 🤷"
)

type ResponseStatus string

const (
	ResponseStatusNeedsAction ResponseStatus = "needsAction"
	ResponseStatusDeclined    ResponseStatus = "declined"
	ResponseStatusTentative   ResponseStatus = "tentative"
	ResponseStatusAccepted    ResponseStatus = "accepted"
)

func (h *Handler) sendEventInvite(account *Account, channel *Channel, event *calendar.Event) error {
	h.stats.Count("sendEventInvite")

	message := `You've been invited to %s: %s
Awaiting your response. *Are you going?*`

	var eventType string
	if event.Recurrence == nil {
		eventType = "an event"
	} else {
		eventType = "a recurring event"
	}

	srv, err := GetCalendarService(account, h.oauth, h.db)
	if err != nil {
		return err
	}
	timezone, err := GetUserTimezone(srv)
	if err != nil {
		return err
	}
	format24HourTime, err := GetUserFormat24HourTime(srv)
	if err != nil {
		return err
	}
	invitedCalendar, err := srv.Calendars.Get(channel.CalendarID).Do()
	if err != nil {
		return err
	}
	eventContent, err := FormatEvent(event, invitedCalendar.Summary, timezone, format24HourTime)
	if err != nil {
		return err
	}

	sendRes, err := h.kbc.SendMessageByTlfName(account.KeybaseUsername, message, eventType, eventContent)
	if err != nil {
		return err
	}

	err = h.db.InsertInvite(account, Invite{
		CalendarID: invitedCalendar.Id,
		EventID:    event.Id,
		MessageID:  *sendRes.Result.MessageID,
	})
	if err != nil {
		return err
	}

	for _, reaction := range []InviteReaction{InviteReactionYes, InviteReactionNo, InviteReactionMaybe} {
		_, err = h.kbc.ReactByChannel(chat1.ChatChannel{Name: account.KeybaseUsername},
			*sendRes.Result.MessageID, string(reaction))
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) updateEventResponseStatus(invite *Invite, account *Account, reaction InviteReaction) error {
	h.stats.Count("updateEventResponseStatus")

	var responseStatus ResponseStatus
	var confirmationMessageStatus string
	switch reaction {
	case InviteReactionYes:
		responseStatus = ResponseStatusAccepted
		confirmationMessageStatus = "Going"
	case InviteReactionNo:
		responseStatus = ResponseStatusDeclined
		confirmationMessageStatus = "Not Going"
	case InviteReactionMaybe:
		responseStatus = ResponseStatusTentative
		confirmationMessageStatus = "Maybe Going"
	default:
		// reaction is not valid for responding to the event
		return nil
	}

	srv, err := GetCalendarService(account, h.oauth, h.db)
	if err != nil {
		return err
	}

	// fetch event
	// TODO(marcel): check if event was deleted
	event, err := srv.Events.Get(invite.CalendarID, invite.EventID).Fields("attendees").Do()
	if err != nil {
		return err
	}

	// update response status on event
	shouldPatch := false
	for index := range event.Attendees {
		if event.Attendees[index].Self {
			event.Attendees[index].ResponseStatus = string(responseStatus)
			shouldPatch = true
			break
		}
	}

	if !shouldPatch {
		return nil
	}

	// patch event to reflect new response status
	event, err = srv.Events.Patch(invite.CalendarID, invite.EventID, event).Fields("summary").Do()
	if err != nil {
		return err
	}

	invitedCalendar, err := srv.Calendars.Get(invite.CalendarID).Do()
	if err != nil {
		return err
	}
	accountCalendar := fmt.Sprintf("%s [%s]", invitedCalendar.Summary, account.AccountNickname)

	_, err = h.kbc.SendMessageByTlfName(account.KeybaseUsername, "I've set your status as *%s* for event *%s* on calendar %s.",
		confirmationMessageStatus, event.Summary, accountCalendar)
	if err != nil {
		return err
	}

	return nil
}

func (h *Handler) syncAllInvites(account *Account, srv *calendar.Service, channelID, calendarID string) {
	syncStart := time.Now()

	var nextSyncToken string
	var events []*calendar.Event
	err := srv.Events.List(calendarID).
		Pages(context.Background(), func(page *calendar.Events) error {
			if page.NextPageToken == "" {
				// set the sync token when the page token is empty
				nextSyncToken = page.NextSyncToken
			}
			events = append(events, page.Items...)
			return nil
		})
	if err != nil {
		h.Errorf("error syncing all invites: %s", err)
		return
	}

	for _, event := range events {
		status := EventStatus(event.Status)

		// if the event is cancelled or there aren't any attendees (ie. the user created the event), skip - it's not an invite
		if status == EventStatusCancelled || event.Attendees == nil {
			continue
		}

		// if the event is recurring, only deal with the underlying recurring event
		if event.RecurringEventId != "" && event.RecurringEventId != event.Id {
			continue
		}

		var end time.Time
		if event.End == nil {
			h.Errorf("empty dates in event")
			continue
		} else if event.End.DateTime != "" {
			// this is a normal event
			end, err = time.Parse(time.RFC3339, event.End.DateTime)
			if err != nil {
				h.Errorf("error parsing time: %s", err)
				continue
			}
		} else if event.End.Date != "" {
			// this is an all day event
			end, err = time.Parse(AllDayDateFormat, event.End.Date)
			if err != nil {
				h.Errorf("error parsing time: %s", err)
				continue
			}
			end = end.Add(-24 * time.Hour) // the google API sets the end day to the day after, so compensate by one day
		} else {
			h.Errorf("invalid end date: %+v", event.End)
			continue
		}

		if time.Now().After(end) {
			// the event has already ended, don't send an invite
			continue
		}

		for _, attendee := range event.Attendees {
			responseStatus := ResponseStatus(attendee.ResponseStatus)
			if attendee.Self && !attendee.Organizer && responseStatus == ResponseStatusNeedsAction {
				err = h.db.InsertInvite(account, Invite{
					CalendarID: calendarID,
					EventID:    event.Id,
				})
				if err != nil {
					h.Errorf("error inserting invite: %s", err)
				}
				break
			}
		}
	}

	err = h.db.UpdateChannelNextSyncToken(channelID, nextSyncToken)
	if err != nil {
		h.Errorf("unable to update sync token:", err)
		return
	}

	h.stats.Value("syncAllInvites - duration - seconds", time.Since(syncStart).Seconds())
}
