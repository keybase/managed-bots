package elastiwatch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewEntryGregor(t *testing.T) {
	e, err := newEntry([]byte(`{"message":"boom","severity":"ERROR","program":"dbcleand","time":"2026-10-22T15:00:00Z"}`))
	require.NoError(t, err)
	require.Equal(t, "boom", e.Message)
	require.Equal(t, "ERROR", e.Severity)
	require.Equal(t, "dbcleand", e.Program)
}

func TestNewEntryKBFSLogrus(t *testing.T) {
	e, err := newEntry([]byte(`{"msg":"block get failed","level":"error","time":"2026-10-22T15:00:00.123456789Z"}`))
	require.NoError(t, err)
	require.Equal(t, "block get failed", e.Message)
	require.Equal(t, "ERROR", e.Severity)
}

func TestNewEntryIced(t *testing.T) {
	e, err := newEntry([]byte(`{"message":"node version: v22.0.0","level":"info","program":"logdumpd","pid":13,"@timestamp":"2026-10-22T15:00:00.000Z"}`))
	require.NoError(t, err)
	require.Equal(t, "node version: v22.0.0", e.Message)
	require.Equal(t, "INFO", e.Severity)
	require.Equal(t, "logdumpd", e.Program)
	require.Equal(t, "2026-10-22T15:00:00.000Z", e.Time)
}

func TestNormalizeSeverityWarnFatal(t *testing.T) {
	require.Equal(t, "WARNING", normalizeSeverity("warn"))
	require.Equal(t, "CRITICAL", normalizeSeverity("fatal"))
}
