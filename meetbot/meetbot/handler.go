package meetbot

import (
	"context"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

type Handler struct {
	*base.DebugOutput

	kbc      *kbchat.API
	db       *DB
	requests *base.OAuthRequests
	config   *oauth2.Config
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(kbc *kbchat.API, db *DB, requests *base.OAuthRequests, config *oauth2.Config) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", kbc),
		kbc:         kbc,
		db:          db,
		requests:    requests,
		config:      config,
	}
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "Hello! I can get you setup with a Google Meet video call anytime, just send me `!meet`."
	return base.HandleNewTeam(h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		h.Debug("skipping non-text message")
		return nil
	}

	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!meet"):
		return h.meetHandler(msg)
	default:
		h.Debug("ignoring unknown command: %q", cmd)
		return nil
	}
}

func (h *Handler) meetHandler(msg chat1.MsgSummary) error {
	err := h.meetHandlerInner(msg)
	switch err.(type) {
	case *googleapi.Error:
		h.Debug("unable to get service %v, deleting credentials and retrying", err)
		// retry auth after nuking stored credentials
		if err := h.db.DeleteToken(base.IdentifierFromMsg(msg)); err != nil {
			return err
		}
		return h.meetHandlerInner(msg)
	default:
		return err
	}
}

func (h *Handler) meetHandlerInner(msg chat1.MsgSummary) error {
	isAdmin, err := base.IsAdmin(h.kbc, msg)
	if err != nil {
		return err
	}
	if !isAdmin {
		_, err = h.kbc.SendMessageByConvID(msg.ConvID, "You have must be an admin to authorize me for a team!")
		return err
	}
	identifier := base.IdentifierFromMsg(msg)
	client, err := base.GetOAuthClient(identifier, msg, h.kbc, h.requests, h.config, h.db,
		"Visit %s\n to authorize me to create events.")
	if err != nil {
		return err
	}
	if client == nil {
		// If we are in a 1-1 conv directly or as a bot user with the sender, skip this message.
		if msg.Channel.MembersType == "team" || !(msg.Sender.Username == msg.Channel.Name || len(strings.Split(msg.Channel.Name, ",")) == 2) {
			_, err = h.kbc.SendMessageByConvID(msg.ConvID,
				"OK! I've sent a message to @%s to authorize me.", msg.Sender.Username)
			return err
		}
		return nil
	}

	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return err
	}

	// Create a bogus event on the primary calendar to host the meeting, it is
	// instantly deleted once the meeting is created.
	requestID, err := base.MakeRequestID()
	if err != nil {
		return err
	}
	event := &calendar.Event{
		Start: &calendar.EventDateTime{
			DateTime: "2015-01-01T00:00:00-00:00",
		},
		End: &calendar.EventDateTime{
			DateTime: "2015-01-01T00:00:00-00:00",
		},
		ConferenceData: &calendar.ConferenceData{
			CreateRequest: &calendar.CreateConferenceRequest{
				RequestId: requestID,
			},
		},
	}

	calendarId := "primary"
	event, err = srv.Events.Insert(calendarId, event).ConferenceDataVersion(1).Do()
	if err != nil {
		h.Debug("meetHandler: unable to create event %v", err)
		return err
	}
	if err := srv.Events.Delete(calendarId, event.Id).Do(); err != nil {
		h.Debug("meetHandler: unable to delete event %v", err)
		return err
	}

	if confData := event.ConferenceData; confData != nil {
		for _, ep := range confData.EntryPoints {
			if ep.EntryPointType == "video" {
				link := ep.Label
				if link == "" {
					// strip protocol to skip unfurl prompt
					link = strings.TrimPrefix(ep.Uri, "https://")
				}
				if link == "" {
					continue
				}
				_, err = h.kbc.SendMessageByConvID(msg.ConvID, link)
				return err
			}
		}
	}

	h.Debug("meetHandler: no event found, conferenceData: %+v", event.ConferenceData)
	_, err = h.kbc.SendMessageByConvID(msg.ConvID, "I wasn't able to create a meeting, please try again.")
	return err
}
