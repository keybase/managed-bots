package triviabot

import (
	"fmt"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type Handler struct {
	*base.DebugOutput
	sync.Mutex

	stats       *base.StatsRegistry
	kbc         *kbchat.API
	debugConfig *base.ChatDebugOutputConfig
	db          *DB
	sessions    map[chat1.ConvIDStr]*session
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig, db *DB) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		stats:       stats.SetPrefix("Handler"),
		kbc:         kbc,
		debugConfig: debugConfig,
		db:          db,
		sessions:    make(map[chat1.ConvIDStr]*session),
	}
}

func (h *Handler) handleStart(cmd string, msg chat1.MsgSummary) {
	h.Lock()
	defer h.Unlock()
	convID := msg.ConvID
	session := newSession(h.kbc, h.debugConfig, h.db, convID)
	doneCb, err := session.start(0)
	if err != nil {
		h.ChatErrorf(convID, "handleState: failed to start: %s", err)
	}
	h.sessions[convID] = session
	base.GoWithRecover(h.DebugOutput, func() {
		<-doneCb
		h.ChatEcho(convID, "Session complete, here are the top players")
		err := h.handleTop(convID)
		if err != nil {
			h.ChatErrorf(msg.ConvID, err.Error())
		}
	})
}

func (h *Handler) handleStop(cmd string, msg chat1.MsgSummary) {
	h.Lock()
	defer h.Unlock()
	convID := msg.ConvID
	session, ok := h.sessions[convID]
	if !ok {
		h.ChatEcho(convID, "No trivia session currently running")
		return
	}
	session.stop()
	delete(h.sessions, convID)
	h.ChatEcho(convID, "Session stopped")
}

func (h *Handler) handleTop(convID chat1.ConvIDStr) error {
	users, err := h.db.TopUsers(convID)
	if err != nil {
		return fmt.Errorf("handleTop: failed to get top users: %s", err)
	}
	var resLines []string
	if len(users) == 0 {
		resLines = []string{"No answers yet"}
	}
	for index, u := range users {
		resLines = append(resLines, fmt.Sprintf("%d. @%s (%d points, %d correct, %d incorrect)",
			index+1, u.username, u.points, u.correct, u.incorrect))
	}
	h.ChatEcho(convID, strings.Join(resLines, "\n"))
	return nil
}

func (h *Handler) handleReset(cmd string, msg chat1.MsgSummary) error {
	convID := msg.ConvID
	if err := h.db.ResetConv(convID); err != nil {
		return fmt.Errorf("handleReset: failed to reset: %s", err)
	}
	h.ChatEcho(convID, "Leaderboard reset")
	return nil
}

func (h *Handler) handleAnswer(convID chat1.ConvIDStr, reaction chat1.MessageReaction, sender string) {
	h.Lock()
	defer h.Unlock()
	session, ok := h.sessions[convID]
	if !ok {
		h.Debug("handleAnswer: no session for convID: %s", convID)
		return
	}
	session.answerCh <- answer{
		selection: base.EmojiToNumber(reaction.Body) - 1,
		msgID:     reaction.MessageID,
		username:  sender,
	}
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "Are you up to the challenge? Try `!trivia begin` to find out."
	return base.HandleNewTeam(h.stats, h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Reaction != nil && msg.Sender.Username != h.kbc.GetUsername() {
		h.handleAnswer(msg.ConvID, *msg.Content.Reaction, msg.Sender.Username)
		return nil
	}
	if msg.Content.Text == nil {
		return nil
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!trivia begin"):
		h.stats.Count("start")
		h.handleStart(cmd, msg)
	case strings.HasPrefix(cmd, "!trivia end"):
		h.stats.Count("stop")
		h.handleStop(cmd, msg)
	case strings.HasPrefix(cmd, "!trivia top"):
		h.stats.Count("top")
		return h.handleTop(msg.ConvID)
	case strings.HasPrefix(cmd, "!trivia reset"):
		h.stats.Count("reset")
		return h.handleReset(cmd, msg)
	}
	return nil
}
