package base

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

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

	shutdownCh chan struct{}

	name         string
	announcement string
	awsOpts      *AWSOptions
	kbc          *kbchat.API
	botAdmins    []string
	multiDBDSN   string
	multi        *multi
}

func NewServer(name, announcement string, awsOpts *AWSOptions, multiDBDSN string) *Server {
	return &Server{
		name:         name,
		announcement: announcement,
		awsOpts:      awsOpts,
		botAdmins:    DefaultBotAdmins,
		shutdownCh:   make(chan struct{}),
		multiDBDSN:   multiDBDSN,
	}
}

func (s *Server) Name() string {
	return s.name
}

func (s *Server) SetBotAdmins(admins []string) {
	s.botAdmins = admins
}

func (s *Server) GoWithRecover(eg *errgroup.Group, f func() error) {
	GoWithRecoverErrGroup(eg, s.DebugOutput, f)
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

func (s *Server) HandleSignals(shutdowners ...Shutdowner) (err error) {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, os.Signal(syscall.SIGTERM))
	sig := <-signalCh
	s.Debug("Received %q, shutting down", sig)
	if err := s.Shutdown(); err != nil {
		s.Debug("Unable to shutdown server: %v", err)
	}
	for _, shutdowner := range shutdowners {
		if shutdowner != nil {
			if err := shutdowner.Shutdown(); err != nil {
				s.Debug("Unable to shutdown shutdowner: %v", err)
			}
		}
	}
	signal.Stop(signalCh)
	close(signalCh)
	return nil
}

func (s *Server) Start(keybaseLoc, home, errReportConv string) (kbc *kbchat.API, err error) {
	if s.kbc, err = kbchat.Start(kbchat.RunOptions{
		KeybaseLocation: keybaseLoc,
		HomeDir:         home,
	}); err != nil {
		return s.kbc, err
	}
	debugConfig := NewChatDebugOutputConfig(s.kbc, errReportConv)
	s.DebugOutput = NewDebugOutput("Server", debugConfig)
	if s.multiDBDSN != "" {
		db, err := sql.Open("mysql", s.multiDBDSN)
		if err != nil {
			s.Errorf("failed to connect to Multi MySQL: %s", err)
			return nil, err
		}
		s.multi = newMulti(s.name, NewDB(db), debugConfig)
	}
	return s.kbc, nil
}

func (s *Server) SendAnnouncement(announcement, running string) (err error) {
	if s.announcement == "" {
		return nil
	}
	defer func() {
		if err != nil {
			s.Debug("SendAnnouncement: failed to announce to %q %v", announcement, err)
		}
	}()
	return SendByConvNameOrID(s.kbc, s.DebugOutput, announcement, running)
}

func (s *Server) Listen(handler Handler) error {
	sub, err := s.kbc.Listen(kbchat.ListenOptions{Convs: true})
	if err != nil {
		s.Errorf("Listen: failed to listen: %s", err)
		return err
	}
	s.Debug("startup success, listening for messages and convs...")
	s.Lock()
	shutdownCh := s.shutdownCh
	s.Unlock()
	eg := &errgroup.Group{}
	s.GoWithRecover(eg, func() error { return s.listenForMsgs(shutdownCh, sub, handler) })
	s.GoWithRecover(eg, func() error { return s.listenForConvs(shutdownCh, sub, handler) })
	s.GoWithRecover(eg, func() error { return s.multi.Heartbeat(shutdownCh) })
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
		if !s.multi.IsLeader() {
			s.Debug("listenForMsgs: ignoring message, not the leader")
			continue
		}

		msg := m.Message
		if msg.Content.Text != nil {
			cmd := strings.TrimSpace(msg.Content.Text.Body)
			switch {
			case strings.HasPrefix(cmd, "!logsend"):
				if err := s.handleLogSend(msg); err != nil {
					s.Errorf("listenForMsgs: unable to handleLogSend: %v", err)
				}
				continue
			case strings.HasPrefix(cmd, "!botlog"):
				if err := s.handleBotLogs(msg); err != nil {
					s.Errorf("listenForMsgs: unable to handleBotLogs: %v", err)
				}
				continue
			case strings.HasPrefix(cmd, "!pprof"):
				if err := s.handlePProf(msg); err != nil {
					s.Errorf("listenForMsgs: unable to handlePProf: %v", err)
				}
				continue
			case strings.HasPrefix(cmd, "!stack"):
				if err := s.handleStack(msg); err != nil {
					s.Errorf("listenForMsgs: unable to handleStack: %v", err)
				}
				continue
			case strings.HasPrefix(cmd, fmt.Sprintf("!%s", feedbackCmd(s.kbc.GetUsername()))):
				if err := s.handleFeedback(msg); err != nil {
					s.Errorf("listenForMsgs: unable to handleFeedback: %v", err)
				}
				continue
			}
		}

		err = handler.HandleCommand(msg)
		switch err := err.(type) {
		case nil, OAuthRequiredError:
		default:
			s.ChatErrorf(msg.ConvID, "listenForMsgs: unable to HandleCommand: %v", err)
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

		if !s.multi.IsLeader() {
			s.Debug("listenForMsgs: ignoring new conv, not the leader")
			continue
		}

		if err := handler.HandleNewConv(c.Conversation); err != nil {
			s.Errorf("listenForConvs: unable to HandleNewConv: %v", err)
		}
	}
}

func (s *Server) allowHiddenCommand(msg chat1.MsgSummary) bool {
	for _, username := range s.botAdmins {
		if username == msg.Sender.Username {
			return true
		}
	}
	return false
}

func (s *Server) handleLogSend(msg chat1.MsgSummary) error {
	if !s.allowHiddenCommand(msg) {
		s.Debug("ignoring log send from @%s, botAdmins: %v",
			msg.Sender.Username, s.botAdmins)
		return nil
	}

	s.ChatEcho(msg.ConvID, "starting a log send...")
	cmd := s.kbc.Command("log", "send", "--no-confirm", "--feedback",
		fmt.Sprintf("managed-bot log requested by @%s", msg.Sender.Username))
	output, err := cmd.StdoutPipe()
	if err != nil {
		s.Errorf("unable to get output pipe: %v", err)
		return err
	}
	if err := cmd.Start(); err != nil {
		s.Errorf("unable to start command: %v", err)
		return err
	}
	outputBytes, err := ioutil.ReadAll(output)
	if err != nil {
		s.Errorf("unable to read ouput: %v", err)
		return err
	}
	if len(outputBytes) > 0 {
		s.ChatEcho(msg.ConvID, "log send output: ```%v```", string(outputBytes))
	}
	if err := cmd.Wait(); err != nil {
		s.Errorf("unable to finish command: %v", err)
		return err
	}
	return nil
}

func (s *Server) handlePProf(msg chat1.MsgSummary) error {
	if !s.allowHiddenCommand(msg) {
		s.Debug("ignoring pprof from @%s, botAdmins: %v",
			msg.Sender.Username, s.botAdmins)
		return nil
	}

	toks, userErr, err := SplitTokens(msg.Content.Text.Body)
	if err != nil {
		return err
	} else if userErr != "" {
		s.ChatEcho(msg.ConvID, userErr)
		return nil
	}
	if len(toks) <= 1 {
		s.Errorf("must specify 'trace', 'cpu' or 'heap'. Try `!pprof cpu -d 5m`")
		return nil
	}
	// drop `!` from `!pprof`
	toks[0] = strings.TrimPrefix(toks[0], "!")
	dur, err := time.ParseDuration(toks[len(toks)-1])
	if err != nil {
		s.Errorf("unable to parse duration using default of 5m: %v", err)
		dur = time.Minute * 5
		toks[len(toks)-1] = dur.String()
	}
	outfile := fmt.Sprintf("/tmp/%s-%d.out", toks[1], time.Now().Unix())
	toks = append(toks, outfile)

	s.ChatEcho(msg.ConvID, "starting pprof... %s", toks)
	cmd := s.kbc.Command(toks...)
	if err := cmd.Run(); err != nil {
		s.Errorf("unable to get run command: %v", err)
		return err
	}
	GoWithRecover(s.DebugOutput, func() {
		time.Sleep(dur + time.Second)
		defer func() {
			// Cleanup after the file is sent.
			time.Sleep(time.Minute)
			s.Debug("cleaning up %s", outfile)
			if err = os.Remove(outfile); err != nil {
				s.Errorf("unable to clean up %s: %v", outfile, err)
			}
		}()
		if _, err := s.kbc.SendAttachmentByConvID(msg.ConvID, outfile, ""); err != nil {
			s.Errorf("unable to send attachment profile: %v", err)
		}
	})
	return nil
}

func (s *Server) handleBotLogs(msg chat1.MsgSummary) error {
	if !s.allowHiddenCommand(msg) {
		s.Debug("ignoring bot log request from @%s, botAdmins: %v",
			msg.Sender.Username, s.botAdmins)
		return nil
	}

	if s.awsOpts == nil {
		return fmt.Errorf("AWS not properly configured")
	}

	s.ChatEcho(msg.ConvID, "fetching logs from cloud watch")
	logs, err := GetLatestCloudwatchLogs(s.awsOpts.AWSRegion, s.awsOpts.CloudWatchLogGroup)
	if err != nil {
		return err
	}
	tld := "private"
	if msg.Channel.MembersType == "team" {
		tld = "team"
	}

	folder := fmt.Sprintf("/keybase/%s/%s/botlogs", tld, msg.Channel.Name)
	if err := exec.Command("keybase", "fs", "mkdir", folder).Run(); err != nil {
		return fmt.Errorf("kbfsOutput: failed to make directory: %s", err)
	}
	fileName := fmt.Sprintf("botlogs-%d.txt", time.Now().Unix())
	filePath := fmt.Sprintf("/tmp/%s", fileName)
	defer os.Remove(filePath)
	if err := ioutil.WriteFile(filePath, []byte(strings.Join(logs, "\n")), 0644); err != nil {
		return fmt.Errorf("kbfsOutput: failed to write log output: %s", err)
	}
	if err := exec.Command("keybase", "fs", "mv", filePath, folder).Run(); err != nil {
		return fmt.Errorf("kbfsOutput: failed to move log output: %s", err)
	}
	destFilePath := fmt.Sprintf("%s/%s", folder, fileName)
	s.ChatEcho(msg.ConvID, "log output: %s", destFilePath)
	return nil
}

func (s *Server) handleStack(msg chat1.MsgSummary) error {
	if !s.allowHiddenCommand(msg) {
		s.Debug("ignoring stack from @%s, botAdmins: %v",
			msg.Sender.Username, s.botAdmins)
		return nil
	}

	buf := make([]byte, 2<<20) // found this used in other calls to runtime.Stack in the go src
	buf = buf[:runtime.Stack(buf, true)]

	tld := "private"
	if msg.Channel.MembersType == "team" {
		tld = "team"
	}

	folder := fmt.Sprintf("/keybase/%s/%s/stack", tld, msg.Channel.Name)
	if err := exec.Command("keybase", "fs", "mkdir", folder).Run(); err != nil {
		return fmt.Errorf("kbfsOutput: failed to make directory: %s", err)
	}
	fileName := fmt.Sprintf("stack-%d.txt", time.Now().Unix())
	filePath := fmt.Sprintf("/tmp/%s", fileName)
	defer os.Remove(filePath)
	if err := ioutil.WriteFile(filePath, buf, 0644); err != nil {
		return fmt.Errorf("kbfsOutput: failed to write stack output: %s", err)
	}
	if err := exec.Command("keybase", "fs", "mv", filePath, folder).Run(); err != nil {
		return fmt.Errorf("kbfsOutput: failed to move stack output: %s", err)
	}
	destFilePath := fmt.Sprintf("%s/%s", folder, fileName)
	s.ChatEcho(msg.ConvID, "stack output: %s", destFilePath)
	return nil
}

func (s *Server) handleFeedback(msg chat1.MsgSummary) error {
	toks := strings.Split(strings.TrimSpace(msg.Content.Text.Body), " ")
	if len(toks) < 3 {
		s.ChatEcho(msg.ConvID, "Woah there @%s, I can't deliver a blank message...not again. What did you want to say?",
			msg.Sender.Username)
	} else {
		body := strings.Join(toks[2:], " ")
		s.Report("Feedback from @%s:\n ```%s```", msg.Sender.Username, body)
		s.ChatEcho(msg.ConvID, "Roger that @%s, passed this along to my humans :robot_face:",
			msg.Sender.Username)
	}
	return nil
}
