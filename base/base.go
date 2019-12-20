package base

import (
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type CommandHandler interface {
	HandleCommand(chat1.MsgSummary) error
}

type Handler struct {
	CommandHandler
	*DebugOutput

	kbc *kbchat.API
}

func NewHandler(kbc *kbchat.API, cmdHandler CommandHandler) *Handler {
	return &Handler{
		DebugOutput:    NewDebugOutput("Handler", kbc),
		CommandHandler: cmdHandler,
		kbc:            kbc,
	}
}

func (h *Handler) Listen() error {
	sub, err := h.kbc.ListenForNewTextMessages()
	if err != nil {
		h.Debug("Listen: failed to listen: %s", err)
		return err
	}
	h.Debug("startup success, listening for messages...")
	for {
		msg, err := sub.Read()
		if err != nil {
			h.Debug("Listen: Read() error: %s", err)
			continue
		}
		if err := h.HandleCommand(msg.Message); err != nil {
			h.Debug("unable to HandleCommand: %v", err)
		}
	}
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
