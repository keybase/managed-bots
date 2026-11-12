package base

import (
	"testing"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/stretchr/testify/require"
)

func TestHiddenCommandAllowed(t *testing.T) {
	t.Parallel()

	dm := chat1.ChatChannel{Name: "joshblum,triviabot", MembersType: "impteamnative"}
	team := chat1.ChatChannel{Name: "keybase", MembersType: "team"}
	admins := []string{"joshblum", "mikem"}
	sender := func(name string) chat1.MsgSummary {
		return chat1.MsgSummary{Sender: chat1.MsgSender{Username: name}}
	}

	cases := []struct {
		name     string
		admins   []string
		msg      chat1.MsgSummary
		expected bool
	}{
		{"admin in direct message", admins, withChannel(sender("joshblum"), dm), true},
		{"admin in team channel", admins, withChannel(sender("joshblum"), team), false},
		{"non admin in direct message", admins, withChannel(sender("alice"), chat1.ChatChannel{Name: "alice,triviabot", MembersType: "impteamnative"}), false},
		{"no admins configured", nil, withChannel(sender("joshblum"), dm), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, isHiddenCommandAllowed(tc.admins, "triviabot", tc.msg))
		})
	}
}

func withChannel(msg chat1.MsgSummary, channel chat1.ChatChannel) chat1.MsgSummary {
	msg.Channel = channel
	return msg
}
