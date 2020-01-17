package gcalbot

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/api/option"

	"github.com/keybase/managed-bots/base"
	"google.golang.org/api/googleapi"

	"google.golang.org/api/calendar/v3"
)

func (h *HTTPSrv) handleEventUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			h.Debug("error in event update webhook: ", err)
		}
	}()

	state := r.Header.Get("X-Goog-Resource-State")
	if state == "sync" {
		// Sync or deleted header, safe to ignore
		return
	}

	channelID := r.Header.Get("X-Goog-Channel-ID")
	resourceID := r.Header.Get("X-Goog-Resource-ID")
	channel, err := h.db.GetChannelByChannelID(channelID)
	if err != nil || channel == nil {
		return
	}

	// sanity check
	if channel.ResourceID != resourceID {
		err = fmt.Errorf("channel and request resourceIDs do not match: %s != %s",
			channel.ResourceID, resourceID)
		return
	}

	token, err := h.db.GetToken(channel.AccountID)
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
		err = fmt.Errorf("error updating events for account ID '%s', cal '%s': %s",
			channel.AccountID, channel.CalendarID, err)
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
				exists, err := h.db.ExistsInvite(channel.AccountID, channel.CalendarID, event.Id)
				if err != nil {
					err = fmt.Errorf("error checking in db for invite: %s", err)
					return
				}
				if !exists {
					// user was recently invited to the event
					// TODO(marcel): deal with recurring events
					err = h.handler.sendEventInvite(channel.AccountID, channel.CalendarID, event)
					if err != nil {
						return
					}
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
	accountID, calendarID string,
) error {
	exists, err := h.db.ExistsChannelByAccountAndCalID(accountID, calendarID)
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
		ChannelID:     channelID,
		AccountID:     accountID,
		CalendarID:    calendarID,
		ResourceID:    res.ResourceId,
		Expiry:        time.Unix(res.Expiration/1e3, 0),
		NextSyncToken: events.NextSyncToken,
	})

	return err
}
