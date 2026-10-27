package elastiwatch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewEntryMessageAndSeverity(t *testing.T) {
	e, err := newEntry([]byte(`{"message":"boom","severity":"ERROR","program":"svc","time":"2026-10-22T15:00:00Z"}`), "")
	require.NoError(t, err)
	require.Equal(t, "boom", e.Message)
	require.Equal(t, "ERROR", e.Severity)
	require.Equal(t, "svc", e.Program)
	require.False(t, e.ignore())
}

func TestNewEntryMsgAndLevelAliases(t *testing.T) {
	e, err := newEntry([]byte(`{"msg":"failed","level":"error","time":"2026-10-22T15:00:00.123456789Z"}`), "")
	require.NoError(t, err)
	require.Equal(t, "failed", e.Message)
	require.Equal(t, "ERROR", e.Severity)
	require.False(t, e.ignore())
}

func TestNewEntryProgramFallbackAndError(t *testing.T) {
	e, err := newEntry([]byte(`{"msg":"op","level":"warning","syslog_program":"worker","error":"not found","time":"2026-10-22T15:00:00Z"}`), "")
	require.NoError(t, err)
	require.Equal(t, "worker", e.Program)
	require.Equal(t, "op not found", e.Message)
	require.Equal(t, "WARNING", e.Severity)
	require.Equal(t, []string{"worker", "op", "not", "found"}, e.words())
}

func TestNewEntryKBFSServiceFromIndex(t *testing.T) {
	e, err := newEntry([]byte(`{"msg":"block get failed","level":"error","syslog_program":"kbfs","section":"Server","time":"2026-10-22T15:00:00.123456789Z"}`), "logstash-mdserver-logs-2026.10.22")
	require.NoError(t, err)
	require.Equal(t, "mdserver", e.Program)
	require.Equal(t, "ERROR", e.Severity)
	require.Equal(t, []string{"mdserver", "block", "get", "failed"}, e.words())
	require.False(t, e.ignore())
}

func TestNewEntrySkipsGenericKBFSSyslogProgram(t *testing.T) {
	e, err := newEntry([]byte(`{"msg":"hi","level":"error","syslog_program":"kbfs","instance":"i-abc"}`), "")
	require.NoError(t, err)
	require.Equal(t, "i-abc", e.Program)
}

func TestNewEntryInstanceAndSectionFallback(t *testing.T) {
	fromInstance, err := newEntry([]byte(`{"message":"hi","instance":"inst-a"}`), "")
	require.NoError(t, err)
	require.Equal(t, "inst-a", fromInstance.Program)

	fromSection, err := newEntry([]byte(`{"message":"hi","section":"sec-b"}`), "")
	require.NoError(t, err)
	require.Equal(t, "sec-b", fromSection.Program)
}

func TestNewEntrySkipsEmptyProgramUID(t *testing.T) {
	e, err := newEntry([]byte(`{"message":"alpha  beta","severity":"ERROR"}`), "")
	require.NoError(t, err)
	require.Equal(t, []string{"alpha", "beta"}, e.words())
}

func TestNewEntryTimestampAlias(t *testing.T) {
	e, err := newEntry([]byte(`{"message":"hello","level":"info","program":"app","@timestamp":"2026-10-22T15:00:00.000Z"}`), "")
	require.NoError(t, err)
	require.Equal(t, "hello", e.Message)
	require.Equal(t, "INFO", e.Severity)
	require.Equal(t, "app", e.Program)
	require.Equal(t, "2026-10-22T15:00:00.000Z", e.Time)
	require.True(t, e.ignore())
}

func TestIgnoreLevelInfoKeepsSeverityInfo(t *testing.T) {
	gregor, err := newEntry([]byte(`{"message":"note","severity":"INFO","program":"dbcleand"}`), "logstash-gregord-logs-2026.10.22")
	require.NoError(t, err)
	require.False(t, gregor.ignore())

	iced, err := newEntry([]byte(`{"message":"node version","level":"info","program":"logdumpd"}`), "logstash-iced-logdumpd-logs-2026.10.22")
	require.NoError(t, err)
	require.True(t, iced.ignore())

	debug, err := newEntry([]byte(`{"message":"trace","severity":"DEBUG","program":"dbcleand"}`), "")
	require.NoError(t, err)
	require.True(t, debug.ignore())
}

func TestServiceFromIndex(t *testing.T) {
	require.Equal(t, "mdserver", serviceFromIndex("logstash-mdserver-logs-2026.10.22"))
	require.Equal(t, "iced-merkle", serviceFromIndex("logstash-iced-merkle-logs-2026.10.22"))
	require.Equal(t, "gregord", serviceFromIndex("logstash-gregord-logs-*"))
	require.Equal(t, "", serviceFromIndex("other-index"))
}

func TestNormalizeSeverityWarnFatal(t *testing.T) {
	require.Equal(t, "WARNING", normalizeSeverity("warn"))
	require.Equal(t, "CRITICAL", normalizeSeverity("fatal"))
}
