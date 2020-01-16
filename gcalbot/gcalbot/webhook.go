package gcalbot

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/keybase/managed-bots/base"
	"google.golang.org/api/googleapi"

	"google.golang.org/api/calendar/v3"
)

type WebhookChannel struct {
	Username        string
	Nickname        string
	CalendarID      string
	NextSyncToken   string
	CalendarService *calendar.Service
}

type WebhookChannels struct {
	channelMap sync.Map
}

func (c *WebhookChannels) Get(channelID string) (*WebhookChannel, bool) {
	val, ok := c.channelMap.Load(channelID)
	if ok {
		return val.(*WebhookChannel), true
	} else {
		return nil, false
	}
}

func (c *WebhookChannels) Set(channelID string, account *WebhookChannel) {
	c.channelMap.Store(channelID, account)
}

func (c *WebhookChannels) Delete(channelID string) {
	c.channelMap.Delete(channelID)
}

func (h *HTTPSrv) handleEventUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	state := r.Header.Get("X-Goog-Resource-State")
	if state == "sync" {
		// Sync or deleted header, safe to ignore
		return
	}
	channelID := r.Header.Get("X-Goog-Channel-Id")

	c, ok := h.webhookChannels.Get(channelID)
	if !ok {
		// h.Debug("error getting channel from channelID '%s'", channelID)
		return
	}

	events, err := c.CalendarService.Events.
		List(c.CalendarID).
		SyncToken(c.NextSyncToken).
		Do()
	if err != nil {
		switch err := err.(type) {
		case *googleapi.Error:
			if err.Code == 410 {
				// TODO(marcel)
				return
			}
		}
		h.Debug("error updating events for user '%s', nick '%s', cal '%s': %s",
			c.Username, c.Nickname, c.CalendarID, err)
	}
	for _, event := range events.Items {
		for _, attendee := range event.Attendees {
			if attendee.Self && !attendee.Organizer &&
				ResponseStatus(attendee.ResponseStatus) == ResponseStatusNeedsAction {
				exists, err := h.db.ExistsInviteForUserEvent(c.Username, c.Nickname, c.CalendarID, event.Id)
				if err != nil {
					h.Debug("error checking in db for invite: %s", err)
					return
				}
				if !exists {
					// user was recently invited to the event
					h.handler.sendEventInvite(c.Username, c.Nickname, c.CalendarID, event)
				}
			}
		}
	}

	w.WriteHeader(200)
}

func (h *Handler) getOrCreateEventChannel(
	srv *calendar.Service,
	username, accountNickname, calendarID string,
) (channelID string, err error) {
	// TODO(marcel): read channel
	ok := false
	if ok {
		return channelID, nil
	}

	// channel not found, create one

	// TODO(marcel): persist channelID
	channelID, err = base.MakeRequestID()
	if err != nil {
		return "", err
	}

	// get all events simply to get the NextSyncToken
	events, err := srv.Events.List(calendarID).Do()
	if err != nil {
		return "", err
	}

	h.webhookChannels.Set(channelID, &WebhookChannel{
		Username:        username,
		Nickname:        accountNickname,
		CalendarID:      calendarID,
		NextSyncToken:   events.NextSyncToken,
		CalendarService: srv,
	})

	// open channel
	_, err = srv.Events.Watch(calendarID, &calendar.Channel{
		Address: fmt.Sprintf("https://%s/gcalbot/events/webhook", h.baseURL),
		Id:      channelID,
		Type:    "web_hook",
	}).Do()
	if err != nil {
		return "", err
	}

	return channelID, err
}
