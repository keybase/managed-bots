package base

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoginTokenPurposeIsBound(t *testing.T) {
	t.Parallel()

	token := MakeLoginToken("secret", "pollbot-login", "alice")
	require.True(t, VerifyLoginToken("secret", "pollbot-login", "alice", token))
	require.False(t, VerifyLoginToken("secret", "gcalbot-login", "alice", token))
	require.False(t, VerifyLoginToken("secret", "pollbot-login", "bob", token))
	require.False(t, VerifyLoginToken("other", "pollbot-login", "alice", token))
}
