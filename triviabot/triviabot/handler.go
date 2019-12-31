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

	kbc      *kbchat.API
	db       *DB
	sessions map[string]*session
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(kbc *kbchat.API, db *DB) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", kbc),
		kbc:         kbc,
		db:          db,
		sessions:    make(map[string]*session),
	}
}

func (h *Handler) handleStart(cmd string, msg chat1.MsgSummary) {
	h.Lock()
	defer h.Unlock()
	convID := msg.ConvID
	session := newSession(h.kbc, h.db, convID)
	doneCb, err := session.start(0)
	if err != nil {
		h.ChatDebug(convID, "handleState: failed to start: %s", err)
	}
	h.sessions[convID] = session
	go func() {
		<-doneCb
		h.ChatEcho(convID, "Session complete, here are the top players")
		h.handleTop(convID)
	}()
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
	h.ChatEcho(convID, "Session stopped")
}

func (h *Handler) handleTop(convID string) {
	users, err := h.db.TopUsers(convID)
	if err != nil {
		h.ChatDebug(convID, "handleTop: failed to get top users: %s", err)
		return
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
}

func (h *Handler) handleReset(cmd string, msg chat1.MsgSummary) {
	convID := msg.ConvID
	if err := h.db.ResetConv(convID); err != nil {
		h.ChatDebug(convID, "handleReset: failed to reset: %s", err)
		return
	}
	h.ChatEcho(convID, "Leaderboard reset")
}

func (h *Handler) handleAnswer(convID string, reaction chat1.MessageReaction, sender string) {
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
	welcomeMsg := "Are you up to the challenge? Try `!triva begin` to find out."
	return base.HandleNewConv(h.DebugOutput, h.kbc, conv, welcomeMsg)
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
		h.handleStart(cmd, msg)
	case strings.HasPrefix(cmd, "!trivia end"):
		h.handleStop(cmd, msg)
	case strings.HasPrefix(cmd, "!trivia top"):
		h.handleTop(msg.ConvID)
	case strings.HasPrefix(cmd, "!trivia reset"):
		h.handleReset(cmd, msg)
	default:
		h.Debug("ignoring unknown command: %q", cmd)
	}
	return nil
}
