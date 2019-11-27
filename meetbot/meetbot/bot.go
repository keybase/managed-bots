package meetbot

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/op/go-logging"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

var log = logging.MustGetLogger("meetbot")

const (
	meetingTrigger = "meet"
)

type Options struct {
	KeybaseLocation string
	Home            string
	HTTPAddr        string
	Announcement    string
}

type BotServer struct {
	sync.Mutex
	opts     Options
	kbc      *kbchat.API
	config   *oauth2.Config
	db       *OAuthDB
	requests map[string]chat1.MsgSummary
}

func NewBotServer(opts Options, config *oauth2.Config, db *OAuthDB) *BotServer {
	return &BotServer{
		opts:     opts,
		config:   config,
		db:       db,
		requests: make(map[string]chat1.MsgSummary),
	}
}

func (s *BotServer) Start() (err error) {
	s.debug("Start(%+v", s.opts)

	http.HandleFunc("/meetbot/oauth", s.oauthHandler)
	go http.ListenAndServe(":8080", nil)

	if s.kbc, err = kbchat.Start(kbchat.RunOptions{
		KeybaseLocation: s.opts.KeybaseLocation,
		HomeDir:         s.opts.Home,
	}); err != nil {
		return err
	}

	if _, err := s.kbc.AdvertiseCommands(s.makeAdvertisement()); err != nil {
		s.debug("advertise error: %s", err)
		return err
	}

	if s.opts.Announcement != "" {
		if err := s.sendAnnouncement(s.opts.Announcement, "I'm running."); err != nil {
			return err
		}
	}

	sub, err := s.kbc.ListenForNewTextMessages()
	if err != nil {
		return err
	}
	s.debug("startup success, listening for messages...")
	for {
		msg, err := sub.Read()
		if err != nil {
			s.debug("Read() error: %v", err)
			continue
		}

		// TODO re-enable
		// if msg.Message.Sender.Username == kbc.GetUsername() {
		// 	continue
		// }
		s.runHandler(msg.Message)
	}
}

func (s *BotServer) oauthHandler(w http.ResponseWriter, r *http.Request) {
	s.debug("oauthHandler")

	var err error
	defer func() {
		if err != nil {
			s.debug("oauthHandler: %v", err)
			fmt.Fprintf(w, "Unable to complete request, please try again!")
		}
	}()

	if r.URL == nil {
		err = fmt.Errorf("r.URL == nil")
		return
	}

	query := r.URL.Query()
	state := query.Get("state")

	s.Lock()
	originatingMsg, ok := s.requests[state]
	delete(s.requests, state)
	s.Unlock()
	if !ok {
		err = fmt.Errorf("state %s not found %v", state, s.requests)
		return
	}

	code := query.Get("code")
	token, err := s.config.Exchange(context.TODO(), code)
	if err != nil {
		return
	}

	if err = s.db.PutToken(identifierFromMsg(originatingMsg), token); err != nil {
		return
	}

	if err = s.meetHandler(originatingMsg); err != nil {
		return
	}

	fmt.Fprintf(w, "Success! You can now close this page return to the Keybase app.")
}

func (s *BotServer) sendAnnouncement(announcement, running string) (err error) {
	defer func() {
		if err == nil {
			s.debug("announcement success")
		}
	}()
	if _, err = s.kbc.SendMessageByConvID(announcement, running); err != nil {
		s.debug("failed to announce self as conv ID: %s", err)
	} else {
		return nil
	}
	if _, err = s.kbc.SendMessageByTlfName(announcement, running); err != nil {
		s.debug("failed to announce self as user: %s", err)
	} else {
		return nil
	}
	if _, err = s.kbc.SendMessageByTeamName(announcement, nil, running); err != nil {
		s.debug("failed to announce self as team: %s", err)
	}
	return err
}

func (s *BotServer) debug(msg string, args ...interface{}) {
	log.Infof("BotServer: "+msg+"\n", args...)
}

func (s *BotServer) isAdmin(msg chat1.MsgSummary) (bool, error) {
	switch msg.Channel.MembersType {
	case "team": // make sure the member is an admin or owner
	default: // authorization is per user so let anything through
		return true, nil
	}

	res, err := s.kbc.ListMembersOfTeam(msg.Channel.Name)
	if err != nil {
		return false, err
	}
	adminLike := append(res.Owners, res.Admins...)
	for _, member := range adminLike {
		if member.Username == msg.Sender.Username {
			return true, nil
		}
	}
	return false, nil
}

func (s *BotServer) getOAuthClient(msg chat1.MsgSummary) (*http.Client, bool, error) {
	identifier := identifierFromMsg(msg)
	token, err := s.db.GetToken(identifier)
	if err != nil {
		return nil, false, err
	}
	// We need to request new authorization
	if token == nil {
		if isAdmin, err := s.isAdmin(msg); err != nil || !isAdmin {
			return nil, isAdmin, err
		}

		state := requestID()
		s.Lock()
		s.requests[state] = msg
		s.Unlock()
		authURL := s.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
		// strip protocol to skip unfurl prompt
		authURL = strings.TrimPrefix(authURL, "https://")
		_, err = s.kbc.SendMessageByTlfName(msg.Sender.Username, "Visit %s\n to authorize me to create events.", authURL)
		return nil, true, err
	}
	return s.config.Client(context.Background(), token), false, nil
}

func (s *BotServer) makeAdvertisement() kbchat.Advertisement {
	return kbchat.Advertisement{
		Alias: "meetbot",
		Advertisements: []chat1.AdvertiseCommandAPIParam{
			{
				Typ: "public",
				Commands: []chat1.UserBotCommandInput{
					{
						Name:        meetingTrigger,
						Description: "Get a URL for a new meet call",
					},
				},
			},
		},
	}
}

func (s *BotServer) runHandler(msg chat1.MsgSummary) {
	convID := msg.ConvID
	var err error
	switch msg.Content.TypeName {
	case "text":
		err = s.textMsgHandler(msg)
	default:
		err = s.logHandler(msg)
	}

	switch err := err.(type) {
	case nil:
		return
	default:
		s.debug("unable to complete request %v", err)
		if _, serr := s.kbc.SendMessageByConvID(convID, "Oh dear, I'm having some trouble. Please try again."); serr != nil {
			s.debug("unable to send: %v", serr)
		}
	}
}

func (s *BotServer) textMsgHandler(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		return s.logHandler(msg)
	}

	cmd := strings.TrimPrefix(strings.TrimSpace(strings.ToLower(strings.Split(msg.Content.Text.Body, " ")[0])), "!")
	switch cmd {
	case meetingTrigger:
		return s.meetHandler(msg)
	default:
		// just log and get out of there
		return s.logHandler(msg)
	}
}

func (s *BotServer) meetHandler(msg chat1.MsgSummary) error {
	err := s.meetHandlerInner(msg)
	switch err.(type) {
	case *googleapi.Error:
		s.debug("unable to get service %v, deleting credentials and retrying", err)
		// retry auth after nuking stored credentials
		if err := s.db.DeleteToken(identifierFromMsg(msg)); err != nil {
			return err
		}
		return s.meetHandlerInner(msg)
	default:
		return err
	}
}

func (s *BotServer) meetHandlerInner(msg chat1.MsgSummary) error {
	client, isAdmin, err := s.getOAuthClient(msg)
	if err != nil {
		return err
	}
	if client == nil {
		if !isAdmin {
			_, err = s.kbc.SendMessageByConvID(msg.ConvID, "You have must be an admin to authorize me for a team!")
			return err
		}
		identifier := identifierFromMsg(msg)
		if identifier != msg.Channel.Name {
			_, err = s.kbc.SendMessageByConvID(msg.ConvID,
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
		s.debug("meetHandler: unable to create event %v", err)
		return err
	}
	if err := srv.Events.Delete(calendarId, event.Id).Do(); err != nil {
		s.debug("meetHandler: unable to delete event %v", err)
		return err
	}

	if confData := event.ConferenceData; confData != nil {
		for _, ep := range confData.EntryPoints {
			if ep.EntryPointType == "video" {
				_, err = s.kbc.SendMessageByConvID(msg.ConvID, "Here you go! %s", ep.Label)
				return err
			}
		}
	}

	s.debug("meetHandler: no event found, conferenceData: %+v", event.ConferenceData)
	_, err = s.kbc.SendMessageByConvID(msg.ConvID, "I wasn't able to create a meeting, please try again.")
	return err
}

func (s *BotServer) logHandler(msg chat1.MsgSummary) error {
	if msg.Content.Text != nil {
		s.debug("unhandled msg from (%s): %s", msg.Sender.Username,
			msg.Content.Text.Body)
	} else {
		s.debug("unhandled msg from (%s): %+v", msg.Sender.Username,
			msg.Content)
	}
	return nil
}
