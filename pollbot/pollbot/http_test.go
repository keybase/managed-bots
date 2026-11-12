package pollbot

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/keybase/managed-bots/base"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret"

func newTestHTTPSrv() *HTTPSrv {
	return &HTTPSrv{
		HTTPSrv:     base.NewHTTPSrv(base.NewStatsRegistryWithBackend(nil, base.NewDummyStatsBackend(nil)), nil),
		tokenSecret: testSecret,
	}
}

func TestVoteCSRFToken(t *testing.T) {
	t.Parallel()

	vote := NewVote("poll1", 2)
	want := voteCSRFToken(testSecret, "alice", vote)
	require.Equal(t, want, voteCSRFToken(testSecret, "alice", vote))
	require.NotEqual(t, want, voteCSRFToken("other", "alice", vote))
	require.NotEqual(t, want, voteCSRFToken(testSecret, "bob", vote))
	require.NotEqual(t, want, voteCSRFToken(testSecret, "alice", NewVote("poll1", 3)))
	require.NotEqual(t, want, voteCSRFToken(testSecret, "alice", NewVote("poll2", 2)))
}

func TestLoginRequiresConfirmation(t *testing.T) {
	t.Parallel()

	h := newTestHTTPSrv()
	token := base.MakeLoginToken(testSecret, "pollbot-login", "alice")

	// A GET from a link must not set the auth cookie. It sets the login CSRF cookie.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/pollbot/login?token="+url.QueryEscape(token)+"&username=alice", nil)
	h.handleLogin(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Log in as @alice")
	require.Contains(t, rec.Body.String(), `name="token"`)
	require.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	csrfCookies := rec.Result().Cookies()
	require.Len(t, csrfCookies, 1)
	require.Equal(t, loginCSRFCookie, csrfCookies[0].Name)
	require.Equal(t, http.SameSiteStrictMode, csrfCookies[0].SameSite)
	csrf := csrfCookies[0].Value
	require.Contains(t, rec.Body.String(), `value="`+csrf+`"`)

	// Only the POST sets the auth cookie, and only with the matching CSRF cookie.
	form := url.Values{"token": {token}, "username": {"alice"}, "csrf": {csrf}}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/pollbot/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(csrfCookies[0])
	h.handleLogin(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, "auth", cookies[0].Name)
	require.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
	require.True(t, cookies[0].HttpOnly)
	require.True(t, cookies[0].Secure)

	// A cross-site form has no CSRF cookie, so the login is rejected even with a valid token.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/pollbot/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.handleLogin(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Empty(t, rec.Header().Values("Set-Cookie"))

	// A token minted for another user is rejected.
	form = url.Values{"token": {token}, "username": {"bob"}, "csrf": {csrf}}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/pollbot/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(csrfCookies[0])
	h.handleLogin(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestVoteConfirmThenPost(t *testing.T) {
	t.Parallel()

	h := newTestHTTPSrv()
	token := base.MakeLoginToken(testSecret, "pollbot-login", "alice")
	vote := NewVote("poll1", 2)
	cookie := &http.Cookie{Name: "auth", Value: "alice:" + token, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}

	// GET renders the confirmation page with the CSRF token and does not record a vote.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/pollbot/vote?="+url.QueryEscape(vote.Encode()), nil)
	req.AddCookie(cookie)
	h.handleVote(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `action="/pollbot/vote"`)
	require.Contains(t, rec.Body.String(), voteCSRFToken(testSecret, "alice", vote))

	// POST with a missing or foreign CSRF token is rejected before any DB access.
	for _, csrf := range []string{"", voteCSRFToken(testSecret, "bob", vote)} {
		form := url.Values{"vote": {vote.Encode()}, "csrf": {csrf}}
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/pollbot/vote", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		h.handleVote(rec, req)
		require.Contains(t, rec.Body.String(), "vote not recorded")
	}
}

func TestVoteWithoutSessionShowsLogin(t *testing.T) {
	t.Parallel()

	h := newTestHTTPSrv()
	vote := NewVote("poll1", 1)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/pollbot/vote?="+url.QueryEscape(vote.Encode()), nil)
	h.handleVote(rec, req)
	require.Contains(t, rec.Body.String(), "Login Required")
	require.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
}
