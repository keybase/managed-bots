package elastiwatch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewEntryMessageAndSeverity(t *testing.T) {
	e, err := newEntry([]byte(`{"message":"boom","severity":"ERROR","program":"svc","time":"2026-10-22T15:00:00Z"}`))
	require.NoError(t, err)
	require.Equal(t, "boom", e.Message)
	require.Equal(t, "ERROR", e.Severity)
	require.Equal(t, "svc", e.Program)
}

func TestNewEntryMsgAndLevelAliases(t *testing.T) {
	e, err := newEntry([]byte(`{"msg":"failed","level":"error","time":"2026-10-22T15:00:00.123456789Z"}`))
	require.NoError(t, err)
	require.Equal(t, "failed", e.Message)
	require.Equal(t, "ERROR", e.Severity)
}

func TestNewEntryProgramFallbackAndError(t *testing.T) {
	e, err := newEntry([]byte(`{"msg":"op","level":"warning","syslog_program":"worker","error":"not found","time":"2026-10-22T15:00:00Z"}`))
	require.NoError(t, err)
	require.Equal(t, "worker", e.Program)
	require.Equal(t, "op not found", e.Message)
	require.Equal(t, "WARNING", e.Severity)
	require.Equal(t, []string{"worker", "op", "not", "found"}, e.words())
}

func TestNewEntryInstanceAndSectionFallback(t *testing.T) {
	fromInstance, err := newEntry([]byte(`{"message":"hi","instance":"inst-a"}`))
	require.NoError(t, err)
	require.Equal(t, "inst-a", fromInstance.Program)

	fromSection, err := newEntry([]byte(`{"message":"hi","section":"sec-b"}`))
	require.NoError(t, err)
	require.Equal(t, "sec-b", fromSection.Program)
}

func TestNewEntrySkipsEmptyProgramUID(t *testing.T) {
	e, err := newEntry([]byte(`{"message":"alpha  beta","severity":"ERROR"}`))
	require.NoError(t, err)
	require.Equal(t, []string{"alpha", "beta"}, e.words())
}

func TestNewEntryTimestampAlias(t *testing.T) {
	e, err := newEntry([]byte(`{"message":"hello","level":"info","program":"app","@timestamp":"2026-10-22T15:00:00.000Z"}`))
	require.NoError(t, err)
	require.Equal(t, "hello", e.Message)
	require.Equal(t, "INFO", e.Severity)
	require.Equal(t, "app", e.Program)
	require.Equal(t, "2026-10-22T15:00:00.000Z", e.Time)
}

func TestNormalizeSeverityWarnFatal(t *testing.T) {
	require.Equal(t, "WARNING", normalizeSeverity("warn"))
	require.Equal(t, "CRITICAL", normalizeSeverity("fatal"))
}
