package base

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"golang.org/x/sync/errgroup"
)

type Handler interface {
	HandleCommand(chat1.MsgSummary) error
	HandleNewConv(chat1.ConvSummary) error
}

type Shutdowner interface {
	Shutdown() error
}

type Server struct {
	*DebugOutput
	sync.Mutex

	shutdownCh   chan struct{}
	announcement string
	kbc          *kbchat.API
	botAdmins    []string
}

func NewServer(announcement string) *Server {
	return &Server{
		announcement: announcement,
		botAdmins:    DefaultBotAdmins,
		shutdownCh:   make(chan struct{}),
	}
}

type OAuthRequests struct {
	sync.Mutex

	Requests map[string]chat1.MsgSummary
}

func NewOAuthRequests() *OAuthRequests {
	return &OAuthRequests{
		Requests: make(map[string]chat1.MsgSummary),
	}
}

func (s *Server) Shutdown() error {
	s.Lock()
	defer s.Unlock()
	if s.shutdownCh != nil {
		close(s.shutdownCh)
		s.shutdownCh = nil
		if err := s.kbc.Shutdown(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) HandleSignals(shutdowner Shutdowner) (err error) {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, os.Signal(syscall.SIGTERM))
	sig := <-signalCh
	s.Debug("Received %q, shutting down", sig)
	if err := s.Shutdown(); err != nil {
		s.Debug("Unable to shutdown server: %v", err)
	}
	if shutdowner != nil {
		if err := shutdowner.Shutdown(); err != nil {
			s.Debug("Unable to shutdown shutdowner: %v", err)
		}
	}
	return nil
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
	s.Debug("startup success, listening for messages and convs...")
	var eg errgroup.Group
	eg.Go(func() error { return s.listenForMsgs(s.shutdownCh, sub, handler) })
	eg.Go(func() error { return s.listenForConvs(s.shutdownCh, sub, handler) })
	if err := eg.Wait(); err != nil {
		s.Debug("wait error: %s", err)
		return err
	}
	s.Debug("Listen: shut down")
	return nil
}

func (s *Server) listenForMsgs(shutdownCh chan struct{}, sub *kbchat.NewSubscription, handler Handler) error {
	for {
		select {
		case <-shutdownCh:
			s.Debug("listenForMsgs: shutting down")
			return nil
		default:
		}

		m, err := sub.Read()
		if err != nil {
			s.Debug("listenForMsgs: Read() error: %s", err)
			continue
		}

		msg := m.Message
		if msg.Content.Text != nil {
			cmd := strings.TrimSpace(msg.Content.Text.Body)
			if strings.HasPrefix(cmd, "!logsend") {
				if err := s.handleLogSend(msg); err != nil {
					s.Debug("listenForMsgs: unable to handleLogSend: %v", err)
				}
				continue
			}
		}

		if err := handler.HandleCommand(msg); err != nil {
			s.Debug("listenForMsgs: unable to HandleCommand: %v", err)
		}
	}
}

func (s *Server) listenForConvs(shutdownCh chan struct{}, sub *kbchat.NewSubscription, handler Handler) error {
	for {
		select {
		case <-shutdownCh:
			s.Debug("listenForConvs: shutting down")
			return nil
		default:
		}

		c, err := sub.ReadNewConvs()
		if err != nil {
			s.Debug("listenForConvs: ReadNewConvs() error: %s", err)
			continue
		}

		if err := handler.HandleNewConv(c.Conversation); err != nil {
			s.Debug("listenForConvs: unable to HandleNewConv: %v", err)
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
