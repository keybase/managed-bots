package meetbot

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

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

	sync.Mutex
	kbc      *kbchat.API
	config   *oauth2.Config
	db       *DB
	requests map[string]chat1.MsgSummary
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(kbc *kbchat.API, config *oauth2.Config, db *DB) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", kbc),
		kbc:         kbc,
		db:          db,
		config:      config,
		requests:    make(map[string]chat1.MsgSummary),
	}
}

func (h *Handler) HTTPListen() error {
	http.HandleFunc("/meetbot", h.healthCheckHandler)
	http.HandleFunc("/meetbot/home", h.homeHandler)
	http.HandleFunc("/meetbot/oauth", h.oauthHandler)
	http.HandleFunc("/meetbot/image", h.handleImage)
	return http.ListenAndServe(":8080", nil)
}

func (h *Handler) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func (h *Handler) homeHandler(w http.ResponseWriter, r *http.Request) {
	h.Debug("homeHandler")
	homePage := `Meetbot is a <a href="https://keybase.io"> Keybase</a> chatbot
	which creates links to Google Meet meetings for you.
	<div style="padding-top:10px;">
		<img style="width:300px;" src="/meetbot/image?=mobile">
	</div>
	`
	if _, err := w.Write(asHTML("home", homePage)); err != nil {
		h.Debug("homeHandler: unable to write: %v", err)
	}
}

func (h *Handler) handleImage(w http.ResponseWriter, r *http.Request) {
	image := r.URL.Query().Get("")
	b64dat, ok := images[image]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	dat, _ := base64.StdEncoding.DecodeString(b64dat)
	if _, err := io.Copy(w, bytes.NewBuffer(dat)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (h *Handler) oauthHandler(w http.ResponseWriter, r *http.Request) {
	h.Debug("oauthHandler")

	var err error
	defer func() {
		if err != nil {
			h.Debug("oauthHandler: %v", err)
			if _, err := w.Write(asHTML("error", "Unable to complete request, please try again!")); err != nil {
				h.Debug("oauthHandler: unable to write: %v", err)
			}
		}
	}()

	if r.URL == nil {
		err = fmt.Errorf("r.URL == nil")
		return
	}

	query := r.URL.Query()
	state := query.Get("state")

	h.Lock()
	originatingMsg, ok := h.requests[state]
	delete(h.requests, state)
	h.Unlock()
	if !ok {
		err = fmt.Errorf("state %q not found %v", state, h.requests)
		return
	}

	code := query.Get("code")
	token, err := h.config.Exchange(context.TODO(), code)
	if err != nil {
		return
	}

	if err = h.db.PutToken(identifierFromMsg(originatingMsg), token); err != nil {
		return
	}

	if err = h.meetHandler(originatingMsg); err != nil {
		return
	}

	if _, err := w.Write(asHTML("success", "Success! You can now close this page and return to the Keybase app.")); err != nil {
		h.Debug("oauthHandler: unable to write: %v", err)
	}
}

func (h *Handler) getOAuthClient(msg chat1.MsgSummary) (*http.Client, bool, error) {
	identifier := identifierFromMsg(msg)
	token, err := h.db.GetToken(identifier)
	if err != nil {
		return nil, false, err
	}
	// We need to request new authorization
	if token == nil {
		if isAdmin, err := base.IsAdmin(h.kbc, msg); err != nil || !isAdmin {
			return nil, isAdmin, err
		}

		state := requestID()
		h.Lock()
		h.requests[state] = msg
		h.Unlock()
		authURL := h.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
		// strip protocol to skip unfurl prompt
		authURL = strings.TrimPrefix(authURL, "https://")
		_, err = h.kbc.SendMessageByTlfName(msg.Sender.Username, "Visit %s\n to authorize me to create events.", authURL)
		return nil, true, err
	}
	return h.config.Client(context.Background(), token), false, nil
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "Hello! I can get you setup with a Google Meet video call anytime, just send me `!meet`."
	return base.HandleNewConv(h.DebugOutput, h.kbc, conv, welcomeMsg)
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
		if err := h.db.DeleteToken(identifierFromMsg(msg)); err != nil {
			return err
		}
		return h.meetHandlerInner(msg)
	default:
		return err
	}
}

func (h *Handler) meetHandlerInner(msg chat1.MsgSummary) error {
	client, isAdmin, err := h.getOAuthClient(msg)
	if err != nil {
		return err
	}
	if client == nil {
		if !isAdmin {
			_, err = h.kbc.SendMessageByConvID(msg.ConvID, "You have must be an admin to authorize me for a team!")
			return err
		}
		// If we are in a 1-1 conv directly or as a bot user with the sender,
		// skip this message.
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
	event := &calendar.Event{
		Start: &calendar.EventDateTime{
			DateTime: "2015-01-01T00:00:00-00:00",
		},
		End: &calendar.EventDateTime{
			DateTime: "2015-01-01T00:00:00-00:00",
		},
		ConferenceData: &calendar.ConferenceData{
			CreateRequest: &calendar.CreateConferenceRequest{
				RequestId: requestID(),
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
