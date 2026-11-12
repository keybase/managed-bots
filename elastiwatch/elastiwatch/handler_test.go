package elastiwatch

import (
	"testing"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/stretchr/testify/require"
)

func TestIsConfiguredTeam(t *testing.T) {
	require.True(t, isConfiguredTeam(chat1.ChatChannel{Name: "ops", MembersType: "team"}, "ops"))
	// a DM or group chat with the same name must not pass the team gate
	require.False(t, isConfiguredTeam(chat1.ChatChannel{Name: "ops", MembersType: "impteam"}, "ops"))
	require.False(t, isConfiguredTeam(chat1.ChatChannel{Name: "ops", MembersType: ""}, "ops"))
	require.False(t, isConfiguredTeam(chat1.ChatChannel{Name: "other", MembersType: "team"}, "ops"))
}
