package base

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestMakeConfirmCode(t *testing.T) {
	t.Parallel()

	code, codeHash, err := MakeConfirmCode()
	require.NoError(t, err)
	require.Regexp(t, `^[A-HJ-NP-Z2-9]{4}-[A-HJ-NP-Z2-9]{4}$`, code)
	require.Equal(t, HashConfirmCode(code), codeHash)

	// users may retype the code loosely
	require.Equal(t, codeHash, HashConfirmCode(strings.ToLower(code)))
	require.Equal(t, codeHash, HashConfirmCode(strings.ReplaceAll(code, "-", "")))
	require.Equal(t, codeHash, HashConfirmCode(strings.ReplaceAll(code, "-", " ")))

	other, _, err := MakeConfirmCode()
	require.NoError(t, err)
	require.NotEqual(t, code, other)
}

func TestParseConfirmCmd(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		body      string
		code      string
		isConfirm bool
	}{
		{"!github confirm ABCD-EFGH", "ABCD-EFGH", true},
		{"!GitHub Confirm  abcd-efgh ", "abcd-efgh", true},
		{"!github confirm ABCD EFGH", "ABCD EFGH", true},
		{"!github confirm", "", true},
		{"!github confirmed ABCD-EFGH", "", false},
		{"!github subscribe a/b", "", false},
		{"!github", "", false},
		{"", "", false},
	} {
		code, isConfirm := ParseConfirmCmd(tc.body, "!github confirm")
		require.Equal(t, tc.isConfirm, isConfirm, tc.body)
		require.Equal(t, tc.code, code, tc.body)
	}
	require.Equal(t, HashConfirmCode("ABCD-EFGH"), HashConfirmCode("ABCD EFGH"))
}

type fakePending struct {
	codeHash string
	token    *oauth2.Token
	attempts int
}

type fakeOAuthStorage struct {
	states  map[string]*OAuthRequest
	pending map[string]*fakePending
	tokens  map[string]*oauth2.Token
	putErr  error
}

var _ OAuthStorage = (*fakeOAuthStorage)(nil)

func newFakeOAuthStorage() *fakeOAuthStorage {
	return &fakeOAuthStorage{
		states:  map[string]*OAuthRequest{},
		pending: map[string]*fakePending{},
		tokens:  map[string]*oauth2.Token{},
	}
}

func (s *fakeOAuthStorage) GetToken(_ context.Context, identifier string) (*oauth2.Token, error) {
	return s.tokens[identifier], nil
}

func (s *fakeOAuthStorage) PutToken(_ context.Context, identifier string, token *oauth2.Token) error {
	if s.putErr != nil {
		return s.putErr
	}
	s.tokens[identifier] = token
	return nil
}

func (s *fakeOAuthStorage) DeleteToken(_ context.Context, identifier string) error {
	delete(s.tokens, identifier)
	return nil
}

func (s *fakeOAuthStorage) GetState(_ context.Context, state string) (*OAuthRequest, error) {
	req, ok := s.states[state]
	if !ok {
		return nil, nil
	}
	cp := *req
	_, cp.IsPending = s.pending[state]
	return &cp, nil
}

func (s *fakeOAuthStorage) PutState(_ context.Context, state string, req *OAuthRequest) error {
	s.states[state] = req
	return nil
}

func (s *fakeOAuthStorage) SetPendingToken(_ context.Context, state, codeHash, pendingToken string) (bool, error) {
	req, ok := s.states[state]
	if !ok || req.IsComplete || s.pending[state] != nil {
		return false, nil
	}
	token, err := UnmarshalPendingToken(pendingToken)
	if err != nil {
		return false, err
	}
	s.pending[state] = &fakePending{codeHash: codeHash, token: token}
	return true, nil
}

func (s *fakeOAuthStorage) ReissueConfirmCode(_ context.Context, state, codeHash string) (bool, error) {
	p, ok := s.pending[state]
	if !ok {
		return false, nil
	}
	p.codeHash = codeHash
	return true, nil
}

func (s *fakeOAuthStorage) GetPendingCodes(_ context.Context, requester string) (res []PendingOAuthCode, err error) {
	for state, p := range s.pending {
		if s.states[state].Requester == requester && p.attempts < MaxConfirmAttempts {
			res = append(res, PendingOAuthCode{State: state, CodeHash: p.codeHash})
		}
	}
	return res, nil
}

func (s *fakeOAuthStorage) GetPendingToken(_ context.Context, state string) (*oauth2.Token, error) {
	if p, ok := s.pending[state]; ok {
		return p.token, nil
	}
	return nil, nil
}

func (s *fakeOAuthStorage) IncrementConfirmAttempts(_ context.Context, requester string) error {
	for state, p := range s.pending {
		if s.states[state].Requester == requester {
			p.attempts++
		}
	}
	return nil
}

func (s *fakeOAuthStorage) CompleteState(_ context.Context, state string) (bool, error) {
	req, ok := s.states[state]
	if !ok || req.IsComplete {
		return false, nil
	}
	req.IsComplete = true
	delete(s.pending, state)
	return true, nil
}

func newPendingStorage(t *testing.T) (storage *fakeOAuthStorage, code string) {
	storage = newFakeOAuthStorage()
	code, codeHash, err := MakeConfirmCode()
	require.NoError(t, err)
	storage.states["s1"] = &OAuthRequest{TokenIdentifier: "alice", Requester: "alice"}
	storage.pending["s1"] = &fakePending{codeHash: codeHash, token: &oauth2.Token{AccessToken: "tok"}}
	return storage, code
}

func TestMatchConfirmCode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	storage, code := newPendingStorage(t)

	t.Run("other requester", func(t *testing.T) {
		state, err := MatchConfirmCode(ctx, storage, "mallory", code)
		require.NoError(t, err)
		require.Empty(t, state)
		require.Zero(t, storage.pending["s1"].attempts)
	})

	t.Run("wrong code counts an attempt", func(t *testing.T) {
		state, err := MatchConfirmCode(ctx, storage, "alice", "AAAA-AAAA")
		require.NoError(t, err)
		require.Empty(t, state)
		require.Equal(t, 1, storage.pending["s1"].attempts)
	})

	t.Run("right code", func(t *testing.T) {
		state, err := MatchConfirmCode(ctx, storage, "alice", strings.ToLower(code))
		require.NoError(t, err)
		require.Equal(t, "s1", state)
	})

	t.Run("locked out after too many attempts", func(t *testing.T) {
		storage.pending["s1"].attempts = MaxConfirmAttempts
		state, err := MatchConfirmCode(ctx, storage, "alice", code)
		require.NoError(t, err)
		require.Empty(t, state)
	})
}

func TestConfirmPendingOAuth(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	save := func(storage *fakeOAuthStorage) func(context.Context, string, *oauth2.Token) error {
		return func(ctx context.Context, _ string, token *oauth2.Token) error {
			return storage.PutToken(ctx, "alice", token)
		}
	}

	t.Run("failed save can be retried with the same code", func(t *testing.T) {
		storage, code := newPendingStorage(t)
		storage.putErr = errors.New("db down")

		_, err := ConfirmPendingOAuth(ctx, storage, "alice", code, save(storage))
		require.Error(t, err)
		require.Contains(t, storage.pending, "s1")
		require.False(t, storage.states["s1"].IsComplete)

		storage.putErr = nil
		confirmed, err := ConfirmPendingOAuth(ctx, storage, "alice", code, save(storage))
		require.NoError(t, err)
		require.True(t, confirmed)
		require.Equal(t, "tok", storage.tokens["alice"].AccessToken)
		require.True(t, storage.states["s1"].IsComplete)

		// single use
		confirmed, err = ConfirmPendingOAuth(ctx, storage, "alice", code, save(storage))
		require.NoError(t, err)
		require.False(t, confirmed)
	})

	t.Run("wrong code saves nothing", func(t *testing.T) {
		storage, _ := newPendingStorage(t)
		confirmed, err := ConfirmPendingOAuth(ctx, storage, "alice", "AAAA-AAAA", save(storage))
		require.NoError(t, err)
		require.False(t, confirmed)
		require.Empty(t, storage.tokens)
	})
}

func TestOAuthStateCookie(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	setOAuthStateCookie(w, "/bot/oauth", "state1")
	setOAuthStateCookie(w, "/bot/oauth", "state2")
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 2)
	for _, c := range cookies {
		require.Equal(t, "/bot/oauth", c.Path)
		require.True(t, c.HttpOnly)
		require.True(t, c.Secure)
		require.Equal(t, http.SameSiteLaxMode, c.SameSite)
	}

	r := httptest.NewRequest(http.MethodGet, "/bot/oauth?state=state1", nil)
	require.False(t, hasOAuthStateCookie(r, "state1"))
	for _, c := range cookies {
		r.AddCookie(c)
	}
	// a second flow in the same browser doesn't displace the first
	require.True(t, hasOAuthStateCookie(r, "state1"))
	require.True(t, hasOAuthStateCookie(r, "state2"))
	require.False(t, hasOAuthStateCookie(r, "state3"))
	require.False(t, hasOAuthStateCookie(r, ""))
}

func TestOAuthStartURL(t *testing.T) {
	t.Parallel()

	for _, redirect := range []string{"https://bots.example/bot/oauth", "https://bots.example/bot/oauth/"} {
		config := &oauth2.Config{RedirectURL: redirect}
		require.Equal(t, "https://bots.example/bot/oauth/start?state=abc", OAuthStartURL(config, "abc"))
		require.Equal(t, "/bot/oauth", oauthCookiePath(config))
	}
}

func TestMakeOAuthStartBodyEscapes(t *testing.T) {
	t.Parallel()

	body := makeOAuthStartBody("bot", "<b>alice</b>", "team<script>", `https://p/auth?a=1&b="x"`)
	require.NotContains(t, body, "<b>alice</b>")
	require.NotContains(t, body, "<script>")
	require.Contains(t, body, "&lt;b&gt;alice&lt;/b&gt;")
	require.Contains(t, body, `href="https://p/auth?a=1&amp;b=&#34;x&#34;"`)
	require.Contains(t, body, "for the team")
	require.NotContains(t, makeOAuthStartBody("bot", "alice", "alice", "u"), "for the team")
}

func newTestOAuthFlow(t *testing.T, storage OAuthFlowStorage) *OAuthFlow {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		if r.Form.Get("code") != "good-code" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"provider-token","token_type":"bearer"}`))
	}))
	t.Cleanup(tokenSrv.Close)

	config := &oauth2.Config{
		ClientID:    "id",
		RedirectURL: "https://bots.example/test/oauth",
		Endpoint:    oauth2.Endpoint{AuthURL: "https://provider.example/authorize", TokenURL: tokenSrv.URL},
	}
	return NewOAuthFlow(NewDebugOutput("test", nil), config, storage, "!test confirm", "testbot-kb",
		"testbot", "/test/image/logo", "/test/oauth")
}

func stateCookie(state string) *http.Cookie {
	return &http.Cookie{
		Name: oauthStateCookiePrefix + state, Value: state,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	}
}

var confirmCodeRE = regexp.MustCompile(`!test confirm ([A-Z0-9]{4}-[A-Z0-9]{4})`)

func TestOAuthFlowStart(t *testing.T) {
	t.Parallel()

	storage := newFakeOAuthStorage()
	storage.states["s1"] = &OAuthRequest{TokenIdentifier: "alice", Requester: "alice"}
	f := newTestOAuthFlow(t, storage)

	t.Run("unknown state", func(t *testing.T) {
		w := httptest.NewRecorder()
		f.HandleStart(w, httptest.NewRequest(http.MethodGet, "/test/oauth/start?state=nope", nil))
		require.Empty(t, w.Result().Cookies())
		require.Contains(t, w.Body.String(), "Unable to complete request")
	})

	t.Run("valid state", func(t *testing.T) {
		w := httptest.NewRecorder()
		f.HandleStart(w, httptest.NewRequest(http.MethodGet, "/test/oauth/start?state=s1", nil))
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		require.Equal(t, oauthStateCookiePrefix+"s1", cookies[0].Name)
		require.Equal(t, "/test/oauth", cookies[0].Path)
		body := w.Body.String()
		require.Contains(t, body, "Keybase user <b>@alice</b>")
		require.Contains(t, body, "through <b>@testbot-kb</b>")
		require.Contains(t, body, "https://provider.example/authorize?")
		require.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	})

	t.Run("POST rejected", func(t *testing.T) {
		w := httptest.NewRecorder()
		f.HandleStart(w, httptest.NewRequest(http.MethodPost, "/test/oauth/start?state=s1", nil))
		require.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}

func TestOAuthFlowCallback(t *testing.T) {
	t.Parallel()

	newStorage := func() *fakeOAuthStorage {
		storage := newFakeOAuthStorage()
		storage.states["s1"] = &OAuthRequest{TokenIdentifier: "alice", Requester: "alice"}
		return storage
	}
	callback := func(f *OAuthFlow, code string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/test/oauth?state=s1&code="+code, nil)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		f.HandleCallback(w, r)
		return w
	}

	t.Run("forwarded link without start cookie", func(t *testing.T) {
		storage := newStorage()
		w := callback(newTestOAuthFlow(t, storage), "good-code", nil)
		require.Contains(t, w.Body.String(), "Unable to complete request")
		require.Empty(t, storage.pending)
	})

	t.Run("cookie for a different state", func(t *testing.T) {
		storage := newStorage()
		callback(newTestOAuthFlow(t, storage), "good-code", stateCookie("s2"))
		require.Empty(t, storage.pending)
	})

	t.Run("bad code from provider", func(t *testing.T) {
		storage := newStorage()
		w := callback(newTestOAuthFlow(t, storage), "bad-code", stateCookie("s1"))
		require.Contains(t, w.Body.String(), "Unable to complete request")
		require.Empty(t, storage.pending)
	})

	t.Run("success, reload, and completion", func(t *testing.T) {
		storage := newStorage()
		f := newTestOAuthFlow(t, storage)

		w := callback(f, "good-code", stateCookie("s1"))
		body := w.Body.String()
		m := confirmCodeRE.FindStringSubmatch(body)
		require.NotNil(t, m, body)
		require.Contains(t, body, `send this message to <a href="keybase://chat/testbot-kb"><b>@testbot-kb</b></a>`)
		require.Contains(t, body, "<b>@alice</b>'s account")
		require.Contains(t, body, "Never send it to anyone else")

		// nothing usable is stored until the requester confirms
		require.Empty(t, storage.tokens)
		require.Equal(t, HashConfirmCode(m[1]), storage.pending["s1"].codeHash)
		require.Equal(t, "provider-token", storage.pending["s1"].token.AccessToken)

		// a reload in the same browser shows a new code; the old one stops working
		w = callback(f, "good-code", stateCookie("s1"))
		m2 := confirmCodeRE.FindStringSubmatch(w.Body.String())
		require.NotNil(t, m2, w.Body.String())
		require.NotEqual(t, m[1], m2[1])
		require.Equal(t, HashConfirmCode(m2[1]), storage.pending["s1"].codeHash)
		require.Equal(t, "provider-token", storage.pending["s1"].token.AccessToken)

		// the callback URL without the cookie still gets nothing
		w = callback(f, "good-code", nil)
		require.Contains(t, w.Body.String(), "Unable to complete request")
		require.Equal(t, HashConfirmCode(m2[1]), storage.pending["s1"].codeHash)

		_, err := storage.CompleteState(context.Background(), "s1")
		require.NoError(t, err)
		w = callback(f, "good-code", stateCookie("s1"))
		require.Contains(t, w.Body.String(), "Success!")
	})
}
