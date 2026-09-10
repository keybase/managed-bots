package gcalbot

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIgnoreAccountAuthError(t *testing.T) {
	t.Parallel()
	authErr := AccountAuthError{Username: "u", Nickname: "n"}
	require.True(t, IsAccountAuthError(authErr))
	require.True(t, IsAccountAuthError(fmt.Errorf("wrap: %w", authErr)))
	require.NoError(t, IgnoreAccountAuthError(authErr))
	require.EqualError(t, IgnoreAccountAuthError(fmt.Errorf("boom")), "boom")
	require.False(t, IsAccountAuthError(fmt.Errorf("boom")))
}
