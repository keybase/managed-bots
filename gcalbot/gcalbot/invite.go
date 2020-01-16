package gcalbot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
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

	err = h.createEventChannel(srv, username, accountNickname, primaryCalendar.Id)
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

	username := msg.Sender.Username
	accountNickname := args[0]
	exists, err := h.db.ExistsAccountForUser(username, accountNickname)
	if err != nil {
		return err
	} else if !exists {
		return nil
	}

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

	channel, err := h.db.GetChannelByUser(username, accountNickname, primaryCalendar.Id)
	if err != nil || channel == nil {
		return err
	}

	err = srv.Channels.Stop(&calendar.Channel{
		Id:         channel.ID,
		ResourceId: channel.ResourceID,
	}).Do()
	if err != nil {
		return err
	}

	err = h.db.DeleteChannelByID(channel.ID)
	if err != nil {
		return err
	}

	_, err = h.kbc.SendMessageByConvID(msg.ConvID,
		"OK, you will no longer be notified of event invites for your primary calendar '%s'.", primaryCalendar.Summary)
	if err != nil {
		return fmt.Errorf("error sending message: %s", err)
	}

	return nil
}

func (h *Handler) sendEventInvite(username, nickname, calendarID string, event *calendar.Event) {
	// TODO(marcel): display which calendar and nickname this is for
	message := `
You've been invited to an event: %s
What: *%s*
When: %s - %s
Awaiting your response. *Are you going?*
`
	messageWithLocation := `
You've been invited to an event: %s
What: *%s*
When: %s - %s
Where: %s
Awaiting your response. *Are you going?*
`

	// strip protocol to skip unfurl prompt
	url := strings.TrimPrefix(event.HtmlLink, "https://")

	startTime, err := time.Parse(time.RFC3339, event.Start.DateTime)
	if err != nil {
		return
	}
	startTimeFormatted := startTime.Format(time.RFC1123)
	endTime, err := time.Parse(time.RFC3339, event.End.DateTime)
	if err != nil {
		return
	}
	endTimeFormatted := endTime.Format(time.RFC1123)

	var sendRes kbchat.SendResponse
	if event.Location != "" {
		sendRes, err = h.kbc.SendMessageByTlfName(username, messageWithLocation,
			url, event.Summary, startTimeFormatted, endTimeFormatted, event.Location)
	} else {
		sendRes, err = h.kbc.SendMessageByTlfName(username, message,
			url, event.Summary, startTimeFormatted, endTimeFormatted)
	}
	if err != nil {
		h.Debug("error sending message: %s", err)
		return
	}

	for _, reaction := range []InviteReaction{InviteReactionYes, InviteReactionNo, InviteReactionMaybe} {
		_, err = h.kbc.ReactByChannel(chat1.ChatChannel{Name: username}, *sendRes.Result.MessageID, string(reaction))
		if err != nil {
			h.Debug("error reacting to message: %s", err)
			return
		}
	}

	err = h.db.InsertInvite(&Invite{
		Username:   username,
		Nickname:   nickname,
		CalendarID: calendarID,
		EventID:    event.Id,
		MessageID:  uint(*sendRes.Result.MessageID),
	})
	if err != nil {
		h.Debug("error inserting invite: %s", err)
	}
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

	identifier := GetAccountIdentifier(invite.Username, invite.Nickname)
	token, err := h.db.GetToken(identifier)
	if err != nil {
		return err
	} else if token == nil {
		h.Debug("token not found for '%s'", identifier)
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
	for index := range event.Attendees {
		if event.Attendees[index].Self {
			event.Attendees[index].ResponseStatus = string(responseStatus)
		}
	}

	// patch event to reflect new response status
	event, err = srv.Events.Patch(invite.CalendarID, invite.EventID, event).Fields("summary").Do()
	if err != nil {
		return err
	}

	// TODO(marcel): specify which account this is for
	_, err = h.kbc.SendMessageByTlfName(invite.Username, "I've set your status as '%s' for event '%s'.",
		confirmationMessageStatus, event.Summary)
	if err != nil {
		return err
	}

	return nil
}
