package gcalbot

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/api/option"

	"github.com/keybase/managed-bots/base"
	"google.golang.org/api/googleapi"

	"google.golang.org/api/calendar/v3"
)

func (h *HTTPSrv) handleEventUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	state := r.Header.Get("X-Goog-Resource-State")
	if state == "sync" {
		// Sync or deleted header, safe to ignore
		return
	}

	channelID := r.Header.Get("X-Goog-Channel-ID")
	resourceID := r.Header.Get("X-Goog-Resource-ID")
	channel, err := h.db.GetChannelByID(channelID)
	if err != nil || channel == nil {
		return
	}
	// sanity check
	if channel.ResourceID != resourceID {
		return
	}

	identifier := GetAccountIdentifier(channel.Username, channel.Nickname)
	token, err := h.db.GetToken(identifier)
	if err != nil {
		return
	}

	client := h.handler.config.Client(context.Background(), token)
	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return
	}

	var events []*calendar.Event
	nextSyncToken := channel.NextSyncToken
	err = srv.Events.
		List(channel.CalendarID).
		SyncToken(channel.NextSyncToken).
		Pages(context.Background(), func(page *calendar.Events) error {
			nextSyncToken = page.NextSyncToken
			events = append(events, page.Items...)
			return nil
		})
	if err != nil {
		switch err := err.(type) {
		case *googleapi.Error:
			if err.Code == 410 {
				// TODO(marcel)
				return
			}
		}
		h.Debug("error updating events for user '%s', nick '%s', cal '%s': %s",
			channel.Username, channel.Nickname, channel.CalendarID, err)
		return
	}

	for _, event := range events {
		if event.RecurringEventId != "" && event.RecurringEventId != event.Id {
			// if the event is recurring, only deal with the underlying recurring event
			continue
		}
		if event.Status == "cancelled" {
			// skip cancelled events
			continue
		}
		for _, attendee := range event.Attendees {
			if attendee.Self && !attendee.Organizer &&
				ResponseStatus(attendee.ResponseStatus) == ResponseStatusNeedsAction {
				exists, err := h.db.ExistsInviteForUserEvent(channel.Username, channel.Nickname, channel.CalendarID, event.Id)
				if err != nil {
					h.Debug("error checking in db for invite: %s", err)
					return
				}
				if !exists {
					// user was recently invited to the event
					// TODO(marcel): deal with recurring events
					h.handler.sendEventInvite(channel.Username, channel.Nickname, channel.CalendarID, event)
				}
			}
		}
	}

	err = h.db.UpdateChannelNextSyncToken(channelID, nextSyncToken)
	if err != nil {
		return
	}

	w.WriteHeader(200)
}

func (h *Handler) createEventChannel(
	srv *calendar.Service,
	username, accountNickname, calendarID string,
) error {
	exists, err := h.db.ExistsChannelForUser(username, accountNickname, calendarID)
	if err != nil {
		return err
	} else if exists {
		return nil
	}

	// channel not found, create one

	channelID, err := base.MakeRequestID()
	if err != nil {
		return err
	}

	// get all events simply to get the NextSyncToken
	events, err := srv.Events.List(calendarID).Do()
	if err != nil {
		return err
	}

	// open channel
	res, err := srv.Events.Watch(calendarID, &calendar.Channel{
		Address: fmt.Sprintf("https://%s/gcalbot/events/webhook", h.baseURL),
		Id:      channelID,
		Type:    "web_hook",
	}).Do()
	if err != nil {
		return err
	}

	err = h.db.InsertChannel(&Channel{
		ID:            channelID,
		Username:      username,
		Nickname:      accountNickname,
		CalendarID:    calendarID,
		ResourceID:    res.ResourceId,
		NextSyncToken: events.NextSyncToken,
	})

	return err
}
