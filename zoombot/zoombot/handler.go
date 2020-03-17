package zoombot

import (
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type Handler struct {
	*base.DebugOutput

	stats  *base.StatsRegistry
	kbc    *kbchat.API
	db     *base.OAuthDB
	config *oauth2.Config
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig,
	db *base.OAuthDB, config *oauth2.Config) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		stats:       stats.SetPrefix("Handler"),
		kbc:         kbc,
		db:          db,
		config:      config,
	}
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "Hello! I can get you set up with a Zoom instant meeting anytime, just send me `!zoom`."
	return base.HandleNewTeam(h.stats, h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleAuth(msg chat1.MsgSummary, _ string) error {
	return h.HandleCommand(msg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		return nil
	}

	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!zoom"):
		h.stats.Count("zoom")
		return h.zoomHandler(msg)
	}
	return nil
}

func (h *Handler) zoomHandler(msg chat1.MsgSummary) error {
	retry := func() error {
		// retry auth after nuking stored credentials
		if err := h.db.DeleteToken(base.IdentifierFromMsg(msg)); err != nil {
			return err
		}
		return h.zoomHandlerInner(msg)
	}
	err := h.zoomHandlerInner(msg)
	switch err.(type) {
	case nil, base.OAuthRequiredError:
		return nil
	default:
		if strings.Contains(err.Error(), "oauth2: cannot fetch token") {
			h.Errorf("unable to get service %v, deleting credentials and retrying", err)
			return retry()
		}
		return err
	}
}

func (h *Handler) zoomHandlerInner(msg chat1.MsgSummary) error {
	identifier := base.IdentifierFromMsg(msg)
	client, err := base.GetOAuthClient(identifier, msg, h.kbc, h.config, h.db,
		base.GetOAuthOpts{
			AuthMessageTemplate:    "Visit '%s' to authorize me to create meeting links.",
			OAuthOfflineAccessType: true,
		})
	if err != nil || client == nil {
		return err
	}

	meeting, err := CreateMeeting(client, currentUserID, &CreateMeetingRequest{
		Type: InstantMeeting,
	})
	if err != nil {
		return err
	}
	h.ChatEcho(msg.ConvID, meeting.JoinURL)

	return nil
}
