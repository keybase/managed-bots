package base

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"golang.org/x/sync/errgroup"
)

type Handler interface {
	HandleCommand(chat1.MsgSummary) error
	HandleNewConv(chat1.ConvSummary) error
}

type Server struct {
	*DebugOutput

	announcement string
	kbc          *kbchat.API
	botAdmins    []string
}

func NewServer(announcement string) *Server {
	return &Server{
		announcement: announcement,
		botAdmins:    DefaultBotAdmins,
	}
}

func (s *Server) SetBotAdmins(admins []string) {
	s.botAdmins = admins
}

func (s *Server) Start(keybaseLoc, home string) (kbc *kbchat.API, err error) {
	if s.kbc, err = kbchat.Start(kbchat.RunOptions{
		KeybaseLocation: keybaseLoc,
		HomeDir:         home,
	}); err != nil {
		return s.kbc, err
	}
	s.DebugOutput = NewDebugOutput("Server", s.kbc)
	return s.kbc, nil
}

func (s *Server) SendAnnouncement(announcement, running string) (err error) {
	if s.announcement == "" {
		return nil
	}
	defer func() {
		if err == nil {
			s.Debug("announcement success")
		}
	}()
	if _, err := s.kbc.SendMessageByConvID(announcement, running); err != nil {
		s.Debug("failed to announce self as conv ID: %s", err)
	} else {
		return nil
	}
	if _, err := s.kbc.SendMessageByTlfName(announcement, running); err != nil {
		s.Debug("failed to announce self as user: %s", err)
	} else {
		return nil
	}
	if _, err := s.kbc.SendMessageByTeamName(announcement, nil, running); err != nil {
		s.Debug("failed to announce self as team: %s", err)
		return err
	} else {
		return nil
	}
}

func (s *Server) Listen(handler Handler) error {
	sub, err := s.kbc.Listen(kbchat.ListenOptions{Convs: true})
	if err != nil {
		s.Debug("Listen: failed to listen: %s", err)
		return err
	}
	defer sub.Shutdown()
	s.Debug("startup success, listening for messages and convs...")
	var eg errgroup.Group
	eg.Go(func() error { return s.listenForMsgs(sub, handler) })
	eg.Go(func() error { return s.listenForConvs(sub, handler) })
	if err := eg.Wait(); err != nil {
		s.Debug("wait error: %s", err)
		return err
	}
	return nil
}

func (s *Server) listenForMsgs(sub kbchat.NewSubscription, handler Handler) error {
	for {
		m, err := sub.Read()
		if err != nil {
			s.Debug("Listen: Read() error: %s", err)
			continue
		}

		msg := m.Message
		if msg.Content.Text != nil {
			cmd := strings.TrimSpace(msg.Content.Text.Body)
			if strings.HasPrefix(cmd, "!logsend") {
				if err := s.handleLogSend(msg); err != nil {
					s.Debug("unable to handleLogSend: %v", err)
				}
				continue
			}
		}

		if err := handler.HandleCommand(msg); err != nil {
			s.Debug("unable to HandleCommand: %v", err)
		}
	}
}

func (s *Server) listenForConvs(sub kbchat.NewSubscription, handler Handler) error {
	for {
		c, err := sub.ReadNewConvs()
		if err != nil {
			s.Debug("Listen: ReadNewConvs() error: %s", err)
			continue
		}

		if err := handler.HandleNewConv(c.Conversation); err != nil {
			s.Debug("unable to HandleNewConv: %v", err)
		}
	}
}

func (s *Server) handleLogSend(msg chat1.MsgSummary) error {
	allowed := false
	sender := msg.Sender.Username
	for _, username := range s.botAdmins {
		if sender == username {
			allowed = true
			break
		}
	}
	if !allowed {
		s.Debug("ignoring log send from @%s, botAdmins: %v", sender, s.botAdmins)
		return nil
	}
	s.ChatEcho(msg.ConvID, "starting a log send...")

	cmd := s.kbc.Command("log", "send", "--no-confirm", "--feedback", fmt.Sprintf("log requested by @%s", sender))
	output, err := cmd.StdoutPipe()
	if err != nil {
		s.ChatDebugFull(msg.ConvID, "unable to get output pipe: %v", err)
		return err
	}
	if err := cmd.Start(); err != nil {
		s.ChatDebugFull(msg.ConvID, "unable to start command: %v", err)
		return err
	}
	outputBytes, err := ioutil.ReadAll(output)
	if err != nil {
		s.ChatDebugFull(msg.ConvID, "unable to read ouput: %v", err)
		return err
	}
	if len(outputBytes) > 0 {
		s.ChatDebugFull(msg.ConvID, "log send output: ```%v```", string(outputBytes))
	}
	if err := cmd.Wait(); err != nil {
		s.ChatDebugFull(msg.ConvID, "unable to finish command: %v", err)
		return err
	}
	return nil
}
