package webhookbot

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/didip/tollbooth/v8"
	"github.com/keybase/managed-bots/base"
	"github.com/stretchr/testify/require"
)

func TestHookLimiterIsPerHook(t *testing.T) {
	lmt := newRateLimiter()

	require.Nil(t, tollbooth.LimitByKeys(lmt, []string{"hook-a"}))
	require.NotNil(t, tollbooth.LimitByKeys(lmt, []string{"hook-a"}))
	require.Nil(t, tollbooth.LimitByKeys(lmt, []string{"hook-b"}))
}

func TestIPLimiterUsesRightmostForwardedFor(t *testing.T) {
	lmt := newIPLimiter()
	newReq := func(forwardedFor string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/webhookbot/hook", nil)
		req.Header.Set("X-Forwarded-For", forwardedFor)
		return req
	}

	require.Nil(t, tollbooth.LimitByRequest(lmt, httptest.NewRecorder(), newReq("192.0.2.1")))
	require.NotNil(t, tollbooth.LimitByRequest(lmt, httptest.NewRecorder(), newReq("192.0.2.1")))
	require.Nil(t, tollbooth.LimitByRequest(lmt, httptest.NewRecorder(), newReq("192.0.2.2")))
	// A client-supplied leftmost entry doesn't change the bucket: the proxy appends the real client last.
	require.NotNil(t, tollbooth.LimitByRequest(lmt, httptest.NewRecorder(), newReq("203.0.113.9, 192.0.2.1")))
}

func TestGetMessageRejectsOversizedBody(t *testing.T) {
	h := &HTTPSrv{HTTPSrv: &base.HTTPSrv{DebugOutput: base.NewDebugOutput("test", nil)}}
	req := httptest.NewRequest(http.MethodPost, "/webhookbot/hook",
		bytes.NewReader(make([]byte, webhookBodyLimit+1)))
	base.LimitRequestBody(httptest.NewRecorder(), req, webhookBodyLimit)

	_, err := h.getMessage(req)

	var maxErr *http.MaxBytesError
	require.True(t, errors.As(err, &maxErr))
}

func TestGetMessageAcceptsBodyAtLimit(t *testing.T) {
	h := &HTTPSrv{HTTPSrv: &base.HTTPSrv{DebugOutput: base.NewDebugOutput("test", nil)}}
	body := bytes.Repeat([]byte("a"), webhookBodyLimit)
	req := httptest.NewRequest(http.MethodPost, "/webhookbot/hook", bytes.NewReader(body))
	base.LimitRequestBody(httptest.NewRecorder(), req, webhookBodyLimit)

	msg, err := h.getMessage(req)

	require.NoError(t, err)
	require.Len(t, msg, webhookBodyLimit)
}
