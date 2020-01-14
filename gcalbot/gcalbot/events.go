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
	if r.Header.Get("X-Goog-Resource-State") == "sync" {
		// Sync header, safe to ignore
		return
	}
	// channel := r.Header.Get("X-Goog-Channel-Id")
	// resource := r.Header.Get("X-Goog-Resource-Id")
	// TODO(marcel): do something with this
}

func (h *Handler) handleSubscribeInvites(msg chat1.MsgSummary, args []string) error {
	if !(msg.Sender.Username == msg.Channel.Name) {
		_, err := h.kbc.SendMessageByConvID(msg.ConvID, "This command can only be run through direct message.")
		if err != nil {
			return fmt.Errorf("error sending message: %s", err)
		}
		return nil
	}

	if len(args) != 2 {
		_, err := h.kbc.SendMessageByConvID(msg.ConvID, "Invalid number of arguments.")
		if err != nil {
			return fmt.Errorf("error sending message: %s", err)
		}
		return nil
	}

	username := msg.Sender.Username
	accountNickname := args[0]
	calendarID := args[1]
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

	requestID, err := base.MakeRequestID()
	if err != nil {
		return err
	}

	_, err = srv.Events.Watch(calendarID, &calendar.Channel{
		Address: fmt.Sprintf("https://%s/gcalbot/events/webhook", h.baseURL),
		Id:      requestID,
		Type:    "web_hook",
	}).Do()
	if err != nil {
		switch googleErr := err.(type) {
		case *googleapi.Error:
			if googleErr.Code == 404 {
				_, err = h.kbc.SendMessageByConvID(msg.ConvID, "No calendar with ID '%s' was found.", calendarID)
				if err != nil {
					return fmt.Errorf("error sending message: %s", err)
				}
				return nil
			}
		}
		return err
	}

	_, err = h.kbc.SendMessageByConvID(msg.ConvID, "OK, you will be notified of event invites from now on.")
	if err != nil {
		return fmt.Errorf("error sending message: %s", err)
	}
	return nil
}
