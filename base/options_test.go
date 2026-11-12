package base

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBotAdminsFromEnvironment(t *testing.T) {
	t.Setenv("BOT_ADMINS", " joshblum, mikem ,")
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	opts := NewOptions()
	require.NoError(t, opts.Parse(fs, []string{"bot"}))
	require.Equal(t, []string{"joshblum", "mikem"}, opts.BotAdmins)
}

func TestBotAdminsDefaultEmpty(t *testing.T) {
	t.Setenv("BOT_ADMINS", "")
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	opts := NewOptions()
	require.NoError(t, opts.Parse(fs, []string{"bot"}))
	require.Empty(t, opts.BotAdmins)
}
