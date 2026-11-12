package base

import (
	"testing"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/stretchr/testify/require"
)

func TestIsDirectPrivateMessage(t *testing.T) {
	t.Parallel()

	dm := func(name string) chat1.ChatChannel {
		return chat1.ChatChannel{Name: name, MembersType: "impteamnative"}
	}
	cases := []struct {
		name     string
		channel  chat1.ChatChannel
		sender   string
		expected bool
	}{
		{"sender and bot", dm("alice,gcalbot"), "alice", true},
		{"bot first", dm("gcalbot,alice"), "alice", true},
		{"self dm", dm("alice"), "alice", true},
		{"substring bot suffix", dm("alice,gcalbotfan"), "alice", false},
		{"substring bot prefix", dm("xgcalbot,alice"), "alice", false},
		{"reader suffix", dm("alice,gcalbot#eve"), "alice", false},
		{"three members", dm("alice,bob,gcalbot"), "alice", false},
		{"two humans", dm("alice,bob"), "alice", false},
		{"team", chat1.ChatChannel{Name: "alice,gcalbot", MembersType: "team"}, "alice", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, IsDirectPrivateMessage("gcalbot", tc.sender, tc.channel))
		})
	}
}
