package gcalbot

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/api/googleapi"

	"github.com/keybase/managed-bots/base"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

func (h *HTTPSrv) handleEventUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	state := r.Header.Get("X-Goog-Resource-State")
	if state == "sync" {
		// Sync or deleted header, safe to ignore
		return
	}
	channelID := r.Header.Get("X-Goog-Channel-Id")

	c, ok := h.webhookChannels.Get(channelID)
	if !ok {
		h.Debug("error getting channel from channelID '%s'", channelID)
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
			if attendee.Self && !attendee.Organizer && attendee.ResponseStatus == "needsAction" {
				// TODO(marcel): this message is sent any time the event is modified, should only send once
				// user was invited to the event
				sendRes, err := h.handler.kbc.SendMessageByTlfName(c.Username, "You've been invited to an event: %s", event.HtmlLink)
				if err != nil {
					h.Debug("error sending message: %s", err)
				}
				for _, reaction := range []string{"Yes 👍", "No 👎", "Maybe 🤷"} {
					_, err = h.kbc.ReactByChannel(chat1.ChatChannel{Name: c.Username}, *sendRes.Result.MessageID, reaction)
					if err != nil {
						h.Debug("error reacting to message: %s", err)
					}
				}
			}
		}
	}

	w.WriteHeader(200)
}

func (h *Handler) handleSubscribeInvites(msg chat1.MsgSummary, args []string) error {
	if !(msg.Sender.Username == msg.Channel.Name) {
		_, err := h.kbc.SendMessageByConvID(msg.ConvID, "This command can only be run through direct message.")
		if err != nil {
			return fmt.Errorf("error sending message: %s", err)
		}
		return nil
	}

	if len(args) != 1 {
		_, err := h.kbc.SendMessageByConvID(msg.ConvID, "Invalid number of arguments.")
		if err != nil {
			return fmt.Errorf("error sending message: %s", err)
		}
		return nil
	}

	username := msg.Sender.Username
	accountNickname := args[0]
	identifier := GetAccountIdentifier(username, accountNickname)

	client, err := base.GetOAuthClient(identifier, msg, h.kbc, h.requests, h.config, h.db,
		h.getAccountOAuthOpts(msg, accountNickname))
	if err != nil || client == nil {
		return err
	}

	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return err
	}

	primaryCalendar, err := getPrimaryCalendar(srv)
	if err != nil {
		return err
	}

	_, err = h.getOrCreateEventChannel(srv, username, accountNickname, primaryCalendar.Id)
	if err != nil {
		return err
	}

	_, err = h.kbc.SendMessageByConvID(msg.ConvID,
		"OK, you will be notified of event invites for your primary calendar '%s' from now on.", primaryCalendar.Summary)
	if err != nil {
		return fmt.Errorf("error sending message: %s", err)
	}
	return nil
}

func (h *Handler) getOrCreateEventChannel(
	srv *calendar.Service,
	username, accountNickname, calendarID string,
) (channelID string, err error) {
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
