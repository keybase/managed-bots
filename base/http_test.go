package base

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecureHandlerSetsHeaders(t *testing.T) {
	t.Parallel()

	called := false
	handler := SecureHandler(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	})
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodGet, "/", nil))

	require.True(t, called)
	require.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
}
