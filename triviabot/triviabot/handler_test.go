package triviabot

import (
	"testing"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"github.com/stretchr/testify/require"
)

func newTestHandler() *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", nil),
		sessions:    make(map[chat1.ConvIDStr]*session),
	}
}

func TestHandleAnswerNeverBlocks(t *testing.T) {
	h := newTestHandler()
	convID := chat1.ConvIDStr("conv")
	s := newSession(nil, nil, nil, convID)
	h.sessions[convID] = s

	// Fill the queue past its capacity; nothing reads answerCh, as after a
	// finished session. Every call must return.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < cap(s.answerCh)+5; i++ {
			h.handleAnswer(convID, chat1.MessageReaction{Body: base.NumberToEmoji(1), MessageID: 1},
				"user")
		}
	}()
	finished := false
	select {
	case <-done:
		finished = true
	case <-time.After(5 * time.Second):
	}
	require.Equal(t, true, finished, "handleAnswer blocked on a full answer queue")
	require.Equal(t, cap(s.answerCh), len(s.answerCh), "expected queue to fill to capacity")
}

func TestRemoveSessionOnlyRemovesSameSession(t *testing.T) {
	h := newTestHandler()
	convID := chat1.ConvIDStr("conv")
	old := newSession(nil, nil, nil, convID)
	h.sessions[convID] = old

	// A newer session replaced the old one; removing the old one must not
	// touch it. Require Same rather than Equal: the two sessions are built
	// identically, so they could compare deeply equal.
	current := newSession(nil, nil, nil, convID)
	h.sessions[convID] = current
	h.removeSession(convID, old)
	require.Same(t, current, h.sessions[convID], "removeSession dropped a session it did not own")

	h.removeSession(convID, current)
	_, ok := h.sessions[convID]
	require.Equal(t, false, ok, "removeSession did not remove the finished session")
}
