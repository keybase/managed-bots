package gitlabbot

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/keybase/managed-bots/base"
	"github.com/stretchr/testify/require"
)

// newTestWebhookSrv has no DB, so any test that reaches the subscription lookup
// panics. The tests below only use requests that must be rejected earlier.
func newTestWebhookSrv() *HTTPSrv {
	return &HTTPSrv{HTTPSrv: &base.HTTPSrv{DebugOutput: base.NewDebugOutput("test", nil)}}
}

func TestHandleWebhookRejectsOversizedBody(t *testing.T) {
	h := newTestWebhookSrv()
	req := httptest.NewRequest(http.MethodPost, "/gitlabbot/webhook",
		bytes.NewReader(make([]byte, webhookBodyLimit+1)))
	rec := httptest.NewRecorder()

	h.handleWebhook(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestHandleWebhookRejectsPayloadWithoutProjectBeforeLookup(t *testing.T) {
	h := newTestWebhookSrv()
	req := httptest.NewRequest(http.MethodPost, "/gitlabbot/webhook",
		bytes.NewReader([]byte(`{"object_kind":"push"}`)))
	rec := httptest.NewRecorder()

	h.handleWebhook(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebhookProjectPath(t *testing.T) {
	repo, err := webhookProjectPath([]byte(`{"project":{"path_with_namespace":"Owner/Repo"},"ref":"x"}`))
	require.NoError(t, err)
	require.Equal(t, "owner/repo", repo)

	_, err = webhookProjectPath([]byte(`not json`))
	require.Error(t, err)
}
