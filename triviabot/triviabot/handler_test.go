package triviabot

import (
	"testing"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
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
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleAnswer blocked on a full answer queue")
	}
	if len(s.answerCh) != cap(s.answerCh) {
		t.Fatalf("expected queue to fill to %d, got %d", cap(s.answerCh), len(s.answerCh))
	}
}

func TestRemoveSessionOnlyRemovesSameSession(t *testing.T) {
	h := newTestHandler()
	convID := chat1.ConvIDStr("conv")
	old := newSession(nil, nil, nil, convID)
	h.sessions[convID] = old

	// A newer session replaced the old one; removing the old one must not
	// touch it.
	current := newSession(nil, nil, nil, convID)
	h.sessions[convID] = current
	h.removeSession(convID, old)
	if h.sessions[convID] != current {
		t.Fatal("removeSession dropped a session it did not own")
	}

	h.removeSession(convID, current)
	if _, ok := h.sessions[convID]; ok {
		t.Fatal("removeSession did not remove the finished session")
	}
}
