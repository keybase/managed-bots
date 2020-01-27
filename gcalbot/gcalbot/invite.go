package gcalbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

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

func (h *Handler) handleInvitesSubscribe(msg chat1.MsgSummary, args []string) error {
	if !base.IsDirectPrivateMessage(h.kbc.GetUsername(), msg) {
		h.ChatEcho(msg.ConvID, "This command can only be run through direct message.")
		return nil
	}

	if len(args) != 1 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments.")
		return nil
	}

	keybaseUsername := msg.Sender.Username
	accountNickname := args[0]
	accountID := GetAccountID(keybaseUsername, accountNickname)

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

	exists, err := h.createSubscription(srv, Subscription{
		AccountID:      accountID,
		CalendarID:     primaryCalendar.Id,
		KeybaseChannel: keybaseUsername,
		MinutesBefore:  0,
		Type:           SubscriptionTypeInvite,
	})
	if err != nil || exists {
		// if no error, subscription exists, short circuit
		return err
	}

	h.ChatEcho(msg.ConvID,
		"OK, you will be notified of event invites for your primary calendar '%s' from now on.", primaryCalendar.Summary)
	return nil
}

func (h *Handler) handleInvitesUnsubscribe(msg chat1.MsgSummary, args []string) error {
	if !base.IsDirectPrivateMessage(h.kbc.GetUsername(), msg) {
		h.ChatEcho(msg.ConvID, "This command can only be run through direct message.")
		return nil
	}

	if len(args) != 1 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments.")
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

	exists, err := h.removeSubscription(srv, Subscription{
		AccountID:      accountID,
		CalendarID:     primaryCalendar.Id,
		KeybaseChannel: keybaseUsername,
		MinutesBefore:  0,
		Type:           SubscriptionTypeInvite,
	})
	if err != nil || !exists {
		// if no error, subscription doesn't exist, short circuit
		return err
	}

	h.ChatEcho(msg.ConvID,
		"OK, you will no longer be notified of event invites for your primary calendar '%s'.", primaryCalendar.Summary)
	return nil
}

func (h *Handler) sendEventInvite(srv *calendar.Service, channel *Channel, event *calendar.Event) error {
	message := `You've been invited to %s: %s
> What: *%s*
> When: %s%s%s%s%s
> Calendar: %s
Awaiting your response. *Are you going?*`

	var eventType string
	if event.Recurrence == nil {
		eventType = "an event"
	} else {
		eventType = "a recurring event"
	}

	// strip protocol to skip unfurl prompt
	url := strings.TrimPrefix(event.HtmlLink, "https://")

	what := event.Summary

	// TODO(marcel): better date formatting for recurring events
	timezone, err := srv.Settings.Get("timezone").Do()
	if err != nil {
		return err
	}
	format24HourTimeSetting, err := srv.Settings.Get("format24HourTime").Do()
	if err != nil {
		return err
	}
	format24HourTime, err := strconv.ParseBool(format24HourTimeSetting.Value)
	if err != nil {
		return err
	}
	when, err := FormatTimeRange(event.Start, event.End, timezone.Value, format24HourTime)
	if err != nil {
		return err
	}

	var where string
	if event.Location != "" {
		where = fmt.Sprintf("\n> Where: %s", event.Location)
	}

	var organizer string
	if event.Organizer.DisplayName != "" && event.Organizer.Email != "" {
		organizer = fmt.Sprintf("\n> Organizer: %s <%s>", event.Organizer.DisplayName, event.Organizer.Email)
	} else if event.Organizer.DisplayName != "" {
		organizer = fmt.Sprintf("\n> Organizer: %s", event.Organizer.DisplayName)
	} else if event.Organizer.Email != "" {
		organizer = fmt.Sprintf("\n> Organizer: %s", event.Organizer.Email)
	}

	var conferenceData string
	if event.ConferenceData != nil {
		for _, entryPoint := range event.ConferenceData.EntryPoints {
			uri := strings.TrimPrefix(entryPoint.Uri, "https://")
			switch entryPoint.EntryPointType {
			case "video", "more":
				conferenceData += fmt.Sprintf("\n> Join online: %s", uri)
			case "phone":
				conferenceData += fmt.Sprintf("\n> Join by phone: %s", entryPoint.Label)
				if entryPoint.Pin != "" {
					conferenceData += fmt.Sprintf(" PIN: %s", entryPoint.Pin)
				}
			case "sip":
				conferenceData += fmt.Sprintf("\n> Join by SIP: %s", entryPoint.Label)
			}
		}
	}

	// note: description can contain HTML
	var description string
	if event.Description != "" {
		description = fmt.Sprintf("\n> Description: %s", event.Description)
	}

	account, err := h.db.GetAccountByAccountID(channel.AccountID)
	if err != nil {
		return err
	}
	invitedCalendar, err := srv.Calendars.Get(channel.CalendarID).Do()
	if err != nil {
		return err
	}
	accountCalendar := fmt.Sprintf("%s [%s]", invitedCalendar.Summary, account.AccountNickname)

	sendRes, err := h.kbc.SendMessageByTlfName(account.KeybaseUsername, message,
		eventType, url, what, when, where, conferenceData, organizer, description, accountCalendar)
	if err != nil {
		return err
	}

	err = h.db.InsertInvite(Invite{
		AccountID:       channel.AccountID,
		CalendarID:      invitedCalendar.Id,
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

	account, err := h.db.GetAccountByAccountID(invite.AccountID)
	if err != nil {
		return err
	}
	invitedCalendar, err := srv.Calendars.Get(invite.CalendarID).Do()
	if err != nil {
		return err
	}
	accountCalendar := fmt.Sprintf("%s [%s]", invitedCalendar.Summary, account.AccountNickname)

	_, err = h.kbc.SendMessageByTlfName(invite.KeybaseUsername, "I've set your status as *%s* for event *%s* on calendar %s.",
		confirmationMessageStatus, event.Summary, accountCalendar)
	if err != nil {
		return err
	}

	return nil
}
