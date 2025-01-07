package zoombot

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type Handler struct {
	*base.DebugOutput

	stats  *base.StatsRegistry
	kbc    *kbchat.API
	db     *DB
	config *oauth2.Config
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig,
	db *DB, config *oauth2.Config) *Handler {
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

func (h *Handler) HandleAuth(msg chat1.MsgSummary, identifier string) error {
	token, err := h.db.GetToken(identifier)
	if err != nil {
		return fmt.Errorf("error getting token: %s", err)
	}
	client := h.config.Client(context.Background(), token)

	user, err := GetUser(client, currentUserID)
	if err != nil {
		return err
	}

	err = h.db.CreateUser(user.ID, user.AccountID, identifier)
	if err != nil {
		return fmt.Errorf("error creating user entry: %s", err)
	}
	return h.HandleCommand(msg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		return nil
	}

	cmd := strings.TrimSpace(msg.Content.Text.Body)
	if strings.HasPrefix(cmd, "!zoom") {
		h.stats.Count("zoom")
		return h.zoomHandler(msg, 0)
	}
	return nil
}

func (h *Handler) zoomHandler(msg chat1.MsgSummary, attempts int) error {
	retry := func() error {
		// retry auth after nuking stored credentials
		if err := h.db.DeleteToken(IdentifierFromMsg(msg)); err != nil {
			return err
		}
		attempts++
		return h.zoomHandlerInner(msg, attempts)
	}
	err := h.zoomHandlerInner(msg, attempts)
	switch err := err.(type) {
	case nil, base.OAuthRequiredError:
		return nil
	case ZoomAPIError:
		if err.Code == invalidTokenCode {
			return retry()
		}
		return err
	default:
		if strings.Contains(err.Error(), "oauth2: cannot fetch token") {
			h.Errorf("unable to get service %v, deleting credentials and retrying", err)
			return retry()
		}
		return err
	}
}

func (h *Handler) zoomHandlerInner(msg chat1.MsgSummary, attempts int) error {
	identifier := IdentifierFromMsg(msg)
	client, err := base.GetOAuthClient(identifier, msg, h.kbc, h.config, h.db,
		base.GetOAuthOpts{
			AuthMessageTemplate:    "Authorize me by clicking this link:\n%s",
			OAuthOfflineAccessType: true,
		})
	if err != nil || client == nil {
		return err
	}

	meeting, err := CreateMeeting(client, currentUserID, &CreateMeetingRequest{
		Type: InstantMeeting,
	})
	switch err := err.(type) {
	case nil:
		h.ChatEcho(msg.ConvID, "%s", meeting.JoinURL)
	case ZoomAPIError:
		if err.Code == http.StatusTooManyRequests {
			if attempts > 5 {
				h.ChatEcho(msg.ConvID, "%s", err.Error())
				return nil
			}
			go func() {
				attempts++
				h.Debug("zoomHandlerInner: retrying attempt #%d: %v", attempts, err)
				time.Sleep(500 * time.Millisecond)
				err := h.zoomHandler(msg, attempts)
				switch err := err.(type) {
				case nil, base.OAuthRequiredError:
				default:
					h.ChatErrorf(msg.ConvID, "zoomHandler unable to create meeting: attempt #%d %v", attempts, err)
				}
			}()
			return nil
		}
	}
	return err
}
