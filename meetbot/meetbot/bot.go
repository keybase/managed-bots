package meetbot

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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
	codeTrigger    = "code"
)

type Options struct {
	KeybaseLocation string
	Home            string
	Announcement    string
}

type BotServer struct {
	opts   Options
	kbc    *kbchat.API
	config *oauth2.Config
	db     *OAuthDB
}

func NewBotServer(opts Options, config *oauth2.Config, db *OAuthDB) *BotServer {
	return &BotServer{
		opts:   opts,
		config: config,
		db:     db,
	}
}

func (s *BotServer) Start() (err error) {
	s.debug("Start(%+v", s.opts)

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
		if _, err := s.kbc.SendMessageByTlfName(s.opts.Announcement, "I'm running."); err != nil {
			s.debug("failed to announce self: %s", err)
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

func (s *BotServer) debug(msg string, args ...interface{}) {
	log.Infof("BotServer: "+msg+"\n", args...)
}

func (s *BotServer) getOAuthClient(msg chat1.MsgSummary) (*http.Client, error) {
	identifier := identifierFromMsg(msg)
	token, err := s.db.GetToken(identifier)
	if err != nil {
		return nil, err
	}
	// We need to request new authorization
	if token == nil {
		authURL := s.config.AuthCodeURL(identifier, oauth2.AccessTypeOffline)
		// strip protocol to skip unfurl prompt
		authURL = strings.TrimPrefix(authURL, "https://")
		// TODO for teams only do this for admins, send as a DM to the user and
		// use the state to know which identifier to store at.
		_, err = s.kbc.SendMessageByConvID(msg.ConvID, fmt.Sprintf("Visit %s\n and then send me the authorization code using the `!code` command and try again", authURL))
		return nil, err
	}
	return s.config.Client(context.Background(), token), nil
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
						Description: fmt.Sprintf("Get a URL for a new meet call"),
					},
					{
						Name:        codeTrigger,
						Description: fmt.Sprintf("Authorize me to have access to your calendar with the code."),
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
	case codeTrigger:
		// TODO instead setup a web endpoint to handle the redirect URL directly.
		return s.codeHandler(msg)
	default:
		// just log and get out of there
		return s.logHandler(msg)
	}
}

func (s *BotServer) codeHandler(msg chat1.MsgSummary) error {
	msgText := strings.Split(msg.Content.Text.Body, " ")
	if len(msgText) != 2 {
		_, err := s.kbc.SendMessageByConvID(msg.ConvID, "I wasn't able to read that code correctly. Please try again.")
		return err
	}
	code := msgText[1]
	token, err := s.config.Exchange(context.TODO(), code)
	if err != nil {
		return err
	}

	if err := s.db.PutToken(identifierFromMsg(msg), token); err != nil {
		return err
	}

	_, err = s.kbc.SendMessageByConvID(msg.ConvID, "Got it! You're all set. Type `!meet` to get a meeting link")
	return err
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
	client, err := s.getOAuthClient(msg)
	if err != nil {
		return err
	}
	if client == nil {
		// TODO uncomment once web endpoint is setup
		// identifier := identifierFromMsg(msg)
		// if identifier != msg.Channel.Name {
		// 	_, err = s.kbc.SendMessageByConvID(msg.ConvID,
		// 		fmt.Sprintf("I've sent a message to @%s to authorize me. After that please try again.", msg.Sender.Username))
		// 	return err
		// }
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
				RequestId: randomID(10),
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
				_, err = s.kbc.SendMessageByConvID(msg.ConvID, fmt.Sprintf("Here you go! %s", ep.Label))
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
