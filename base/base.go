package base

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type CommandHandler interface {
	HandleCommand(chat1.MsgSummary) error
}

type Handler struct {
	*DebugOutput
	CommandHandler

	kbc       *kbchat.API
	botAdmins []string
}

func NewHandler(kbc *kbchat.API, cmdHandler CommandHandler) *Handler {
	return &Handler{
		DebugOutput:    NewDebugOutput("Handler", kbc),
		CommandHandler: cmdHandler,
		kbc:            kbc,
		botAdmins:      DefaultBotAdmins,
	}
}

func (h *Handler) SetBotAdmins(admins []string) {
	h.botAdmins = admins
}

func (h *Handler) HandleCommands(msg chat1.MsgSummary) error {
	return errors.New("Not implemented")
}

func (h *Handler) BotAdmins() []string {
	return h.botAdmins
}

func (h *Handler) Listen() error {
	sub, err := h.kbc.ListenForNewTextMessages()
	if err != nil {
		h.Debug("Listen: failed to listen: %s", err)
		return err
	}
	h.Debug("startup success, listening for messages...")
	for {
		m, err := sub.Read()
		if err != nil {
			h.Debug("Listen: Read() error: %s", err)
			continue
		}

		msg := m.Message
		if msg.Content.Text != nil {
			cmd := strings.TrimSpace(msg.Content.Text.Body)
			if strings.HasPrefix(cmd, "!logsend") {
				if err := h.handleLogSend(msg); err != nil {
					h.Debug("unable to handleLogSend: %v", err)
				}
				continue
			}
		}

		if err := h.HandleCommand(msg); err != nil {
			h.Debug("unable to HandleCommand: %v", err)
		}
	}
}

func (h *Handler) handleLogSend(msg chat1.MsgSummary) error {
	allowed := false
	sender := msg.Sender.Username
	for _, username := range h.botAdmins {
		if sender == username {
			allowed = true
			break
		}
	}
	if !allowed {
		h.Debug("ignoring log send from @%s, botAdmins: %v", sender, h.botAdmins)
		return nil
	}
	h.ChatEcho(msg.ConvID, "starting a log send...")

	cmd := h.kbc.Command("log", "send", "--no-confirm", "--feedback", fmt.Sprintf("log requested by @%s", sender))
	output, err := cmd.StdoutPipe()
	if err != nil {
		h.ChatDebugFull(msg.ConvID, "unable to get output pipe: %v", err)
		return err
	}
	if err := cmd.Start(); err != nil {
		h.ChatDebugFull(msg.ConvID, "unable to start command: %v", err)
		return err
	}
	outputBytes, err := ioutil.ReadAll(output)
	if err != nil {
		h.ChatDebugFull(msg.ConvID, "unable to read ouput: %v", err)
		return err
	}
	if len(outputBytes) > 0 {
		h.ChatDebugFull(msg.ConvID, "log send output: ```%v```", string(outputBytes))
	}
	if err := cmd.Wait(); err != nil {
		h.ChatDebugFull(msg.ConvID, "unable to finish command: %v", err)
		return err
	}
	return nil
}

func (h *Handler) IsAdmin(msg chat1.MsgSummary) (bool, error) {
	switch msg.Channel.MembersType {
	case "team": // make sure the member is an admin or owner
	default: // authorization is per user so let anything through
		return true, nil
	}
	res, err := h.kbc.ListMembersOfTeam(msg.Channel.Name)
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

type Server struct {
	*DebugOutput

	announcement string
	kbc          *kbchat.API
}

func NewServer(announcement string) *Server {
	return &Server{
		announcement: announcement,
	}
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
