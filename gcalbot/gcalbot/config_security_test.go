package gcalbot

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"github.com/stretchr/testify/require"
)

// newTestHTTPSrv returns a server with no Keybase or DB connections. Only the
// auth and CSRF checks run before those are used, so the tests stay on those paths.
func newTestHTTPSrv() *HTTPSrv {
	stats := base.NewStatsRegistryWithBackend(nil, base.NewDummyStatsBackend(nil))
	return &HTTPSrv{
		HTTPSrv: base.NewHTTPSrv(stats, nil),
		handler: &Handler{tokenSecret: "test-secret"},
	}
}

// authTestCookie builds an auth cookie with the attributes the server sets.
func authTestCookie(value string) *http.Cookie {
	return &http.Cookie{Name: "auth", Value: value, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
}

func TestIsConvParticipant(t *testing.T) {
	cases := []struct {
		name     string
		username string
		channel  chat1.ChatChannel
		expected bool
	}{
		{"dm member", "alice", chat1.ChatChannel{Name: "alice,gcalbot"}, true},
		{"dm other member", "gcalbot", chat1.ChatChannel{Name: "alice,gcalbot"}, true},
		{"self dm", "alice", chat1.ChatChannel{Name: "alice"}, true},
		{"group member", "bob", chat1.ChatChannel{Name: "alice,bob,gcalbot"}, true},
		{"not a member", "mallory", chat1.ChatChannel{Name: "alice,bob,gcalbot"}, false},
		{"substring is not membership", "ali", chat1.ChatChannel{Name: "alice,gcalbot"}, false},
		{"public reader", "carol", chat1.ChatChannel{Name: "alice,gcalbot#carol"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.expected, isConvParticipant(c.username, c.channel))
		})
	}
}

func TestAuthFromCookie(t *testing.T) {
	h := newTestHTTPSrv()
	token := base.MakeLoginToken(h.handler.tokenSecret, loginPurpose, "alice:conv1")
	cases := []struct {
		name   string
		cookie string
		ok     bool
	}{
		{"valid", "alice:conv1:" + token, true},
		{"token for another user", "bob:conv1:" + token, false},
		{"token for another conv", "alice:conv2:" + token, false},
		{"malformed", "alice:conv1", false},
		{"forged token", "alice:conv1:0.00", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/gcalbot", nil)
			req.AddCookie(authTestCookie(c.cookie))
			_, ok := h.authFromCookie(req)
			require.Equal(t, c.ok, ok)
		})
	}
}

func TestAuthFromLink(t *testing.T) {
	h := newTestHTTPSrv()
	token := base.MakeLoginToken(h.handler.tokenSecret, loginPurpose, "alice:conv1")
	cases := []struct {
		name  string
		query string
		ok    bool
	}{
		{"valid", "username=alice&conv_id=conv1&token=" + token, true},
		{"missing conv", "username=alice&token=" + token, false},
		{"token for another conv", "username=alice&conv_id=conv2&token=" + token, false},
		{"forged token", "username=alice&conv_id=conv1&token=0.00", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/gcalbot?"+c.query, nil)
			w := httptest.NewRecorder()
			_, ok := h.authFromLink(w, req)
			require.Equal(t, c.ok, ok)

			cookies := w.Result().Cookies()
			if !c.ok {
				require.Empty(t, cookies)
				return
			}
			require.Len(t, cookies, 1)
			require.Equal(t, "auth", cookies[0].Name)
			require.True(t, cookies[0].HttpOnly)
			require.True(t, cookies[0].Secure)
			require.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
		})
	}
}

// Writes must not get past the auth and CSRF checks, which both run before any
// Keybase or DB call. A panic here would mean a write reached that code unchecked.
func TestConfigPostRejectsMissingOrBadCSRF(t *testing.T) {
	h := newTestHTTPSrv()
	secret := h.handler.tokenSecret
	token := base.MakeLoginToken(secret, loginPurpose, "alice:conv1")
	otherToken := base.MakeLoginToken(secret, loginPurpose, "bob:conv1")
	cases := []struct {
		name        string
		cookie      bool
		csrf        string
		wantMessage bool
	}{
		{"no session", false, "", false},
		{"missing csrf", true, "", true},
		{"bad csrf", true, "not-a-token", true},
		// A CSRF token is bound to the session it was rendered for.
		{"csrf from another session", true, base.MakeLoginToken(secret, csrfPurpose, otherToken), true},
		// The login token itself is not a CSRF token.
		{"login token as csrf", true, token, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := newConfigPost(c.csrf)
			if c.cookie {
				req.AddCookie(authTestCookie("alice:conv1:" + token))
			}
			w := httptest.NewRecorder()
			h.configHandler(w, req)

			require.Equal(t, http.StatusForbidden, w.Code)
			require.Equal(t, c.wantMessage, strings.Contains(w.Body.String(), "Form expired"))
		})
	}

	// Positive control, so the cases above can't pass by rejecting everything:
	// the session's own CSRF token gets past the check and reaches the Keybase
	// call, which panics on the test server's nil client.
	req := newConfigPost(base.MakeLoginToken(secret, csrfPurpose, token))
	req.AddCookie(authTestCookie("alice:conv1:" + token))
	require.Panics(t, func() { h.configHandler(httptest.NewRecorder(), req) })
}

func newConfigPost(csrf string) *http.Request {
	form := url.Values{"conv_id": {"conv1"}, "csrf_token": {csrf}}
	req := httptest.NewRequest(http.MethodPost, "/gcalbot", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}
