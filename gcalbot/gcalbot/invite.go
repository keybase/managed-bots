package gcalbot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/googleapi"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
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

func (h *Handler) handleSubscribeInvites(msg chat1.MsgSummary, args []string) error {
	if !(msg.Sender.Username == msg.Channel.Name || len(strings.Split(msg.Channel.Name, ",")) == 2) {
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
	accountID := GetAccountID(username, accountNickname)

	client, err := base.GetOAuthClient(accountID, msg, h.kbc, h.requests, h.config, h.db,
		h.getAccountOAuthOpts(msg, accountNickname))
	if err != nil || client == nil {
		// if no error, account doesn't exist, short circuit
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

	subscription := Subscription{
		AccountID:  accountID,
		CalendarID: primaryCalendar.Id,
		Type:       SubscriptionTypeInvite,
	}

	exists, err := h.db.ExistsSubscription(subscription)
	if err != nil || exists {
		// if no error, subscription exists, short circuit
		return err
	}

	err = h.createEventChannel(srv, accountID, primaryCalendar.Id)
	if err != nil {
		return err
	}

	err = h.db.InsertSubscription(subscription)
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

func (h *Handler) handleUnsubscribeInvites(msg chat1.MsgSummary, args []string) error {
	if !(msg.Sender.Username == msg.Channel.Name || len(strings.Split(msg.Channel.Name, ",")) == 2) {
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

	exists, err := h.db.DeleteSubscription(Subscription{
		AccountID:  accountID,
		CalendarID: primaryCalendar.Id,
		Type:       SubscriptionTypeInvite,
	})
	if err != nil || !exists {
		// if no error, subscription doesn't exist, short circuit
		return err
	}

	subscriptionCount, err := h.db.CountSubscriptionsByAccountAndCalID(accountID, primaryCalendar.Id)
	if err != nil {
		return err
	}

	if subscriptionCount == 0 {
		// if there are no more subscriptions for this account + calendar, remove the channel
		channel, err := h.db.GetChannelByAccountAndCalendarID(accountID, primaryCalendar.Id)
		if err != nil {
			return err
		}

		if channel != nil {
			// TODO(marcel): exponential backoff for api calls
			err = srv.Channels.Stop(&calendar.Channel{
				Id:         channel.ChannelID,
				ResourceId: channel.ResourceID,
			}).Do()
			switch err := err.(type) {
			case nil:
			case *googleapi.Error:
				if err.Code != 404 {
					return err
				}
			default:
				return err
			}

			err = h.db.DeleteChannelByChannelID(channel.ChannelID)
			if err != nil {
				return err
			}
		}
	}

	_, err = h.kbc.SendMessageByConvID(msg.ConvID,
		"OK, you will no longer be notified of event invites for your primary calendar '%s'.", primaryCalendar.Summary)
	if err != nil {
		return fmt.Errorf("error sending message: %s", err)
	}

	return nil
}

func (h *Handler) sendEventInvite(accountID, calendarID string, event *calendar.Event) error {
	// TODO(marcel): display which calendar and nickname this is for
	// TODO(marcel): better date formatting
	// TODO(marcel): possibly sanitize titles/info
	var location string
	if event.Location != "" {
		location = fmt.Sprintf("Where: %s\n", event.Location)
	}
	message := `
You've been invited to an event: %s
What: *%s*
When: %s
` + location +
		`Awaiting your response. *Are you going?*`

	account, err := h.db.GetAccountByAccountID(accountID)
	if err != nil {
		return err
	}

	// strip protocol to skip unfurl prompt
	url := strings.TrimPrefix(event.HtmlLink, "https://")

	startTime, err := time.Parse(time.RFC3339, event.Start.DateTime)
	if err != nil {
		return err
	}
	endTime, err := time.Parse(time.RFC3339, event.End.DateTime)
	if err != nil {
		return err
	}
	timeRange := FormatTimeRange(startTime, endTime)

	sendRes, err := h.kbc.SendMessageByTlfName(account.KeybaseUsername, message,
		url, event.Summary, timeRange)
	if err != nil {
		return err
	}

	err = h.db.InsertInvite(Invite{
		AccountID:       accountID,
		CalendarID:      calendarID,
		EventID:         event.Id,
		KeybaseUsername: account.KeybaseUsername,
		MessageID:       uint(*sendRes.Result.MessageID),
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

func (h *Handler) updateEventResponseStatus(invite *Invite, reaction InviteReaction) error {
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

	token, err := h.db.GetToken(invite.AccountID)
	if err != nil {
		return err
	} else if token == nil {
		h.Debug("token not found for '%s'", invite.AccountID)
		return nil
	}

	client := h.config.Client(context.Background(), token)
	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
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

	// TODO(marcel): specify which account this is for
	_, err = h.kbc.SendMessageByTlfName(invite.KeybaseUsername, "I've set your status as '%s' for event '%s'.",
		confirmationMessageStatus, event.Summary)
	if err != nil {
		return err
	}

	return nil
}
