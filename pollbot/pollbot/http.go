package pollbot

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

// formBodyLimit caps the POST forms for login and vote; both carry a few hundred bytes.
const formBodyLimit = 4 << 10

const (
	// loginCSRFCookie pairs a login confirm page with the POST that completes it.
	loginCSRFCookie = "login_csrf"
	// loginConfirmMaxAge is how long a confirm page stays valid.
	loginConfirmMaxAge = 15 * time.Minute
)

type HTTPSrv struct {
	*base.HTTPSrv

	kbc *kbchat.API
	db  *DB

	tokenSecret string
}

func NewHTTPSrv(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig,
	db *DB,
	tokenSecret string,
) *HTTPSrv {
	h := &HTTPSrv{
		kbc:         kbc,
		db:          db,
		tokenSecret: tokenSecret,
	}
	http.HandleFunc("/pollbot", h.handleHealthCheck)
	http.HandleFunc("/pollbot/vote", h.handleVote)
	http.HandleFunc("/pollbot/login", h.handleLogin)
	http.HandleFunc("/pollbot/image", h.handleImage)
	h.HTTPSrv = base.NewHTTPSrv(stats, debugConfig)
	return h
}

func (h *HTTPSrv) showLoginInstructions(w http.ResponseWriter) {
	_, _ = w.Write([]byte(htmlLogin))
}

func (h *HTTPSrv) showSuccess(w http.ResponseWriter) {
	_, _ = w.Write([]byte(makeHTMLVoteResult("Vote success!")))
}

func (h *HTTPSrv) showError(w http.ResponseWriter) {
	_, _ = w.Write([]byte(makeHTMLVoteResult("Something went wrong, vote not recorded.")))
}

// checkLogin returns the username of a valid session.
func (h *HTTPSrv) checkLogin(w http.ResponseWriter, r *http.Request) (string, bool) {
	cookie, err := r.Cookie("auth")
	if err != nil {
		h.Debug("error getting cookie: %s", err)
		h.showLoginInstructions(w)
		return "", false
	}
	if cookie == nil {
		h.Debug("no cookie")
		h.showLoginInstructions(w)
		return "", false
	}
	auth := cookie.Value
	toks := strings.SplitN(auth, ":", 2)
	if len(toks) != 2 {
		h.Debug("malformed auth cookie %v", auth)
		h.showLoginInstructions(w)
		return "", false
	}
	username := toks[0]
	token := toks[1]
	if !base.VerifyLoginToken(h.tokenSecret, "pollbot-login", username, token) {
		h.Debug("invalid auth cookie")
		h.showLoginInstructions(w)
		return "", false
	}
	return username, true
}

// voteCSRFToken binds a vote to the user shown the confirm page,
// so a cross-site form can't cast a vote for the user.
func voteCSRFToken(secret, username string, vote Vote) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(fmt.Appendf(nil, "pollbot-vote-csrf\x00%s\x00%s\x00%d", username, vote.ID, vote.Choice))
	return hex.EncodeToString(mac.Sum(nil))
}

// handleVote shows a confirmation page for a vote link and records the vote only on POST.
func (h *HTTPSrv) handleVote(w http.ResponseWriter, r *http.Request) {
	base.SecureHeaders(w)
	username, ok := h.checkLogin(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.showVoteConfirm(w, r, username)
	case http.MethodPost:
		h.recordVote(w, r, username)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *HTTPSrv) showVoteConfirm(w http.ResponseWriter, r *http.Request, username string) {
	vote, err := NewVoteFromEncoded(r.URL.Query().Get(""))
	if err != nil {
		h.Debug("invalid vote payload: %s", err)
		h.showError(w)
		return
	}
	data := confirmData{
		Title:        "Vote",
		Heading:      "Confirm your vote",
		Instructions: fmt.Sprintf("You are about to vote for option %d.", vote.Choice),
		Action:       "/pollbot/vote",
		Fields: []confirmField{
			{Name: "vote", Value: vote.Encode()},
			{Name: "csrf", Value: voteCSRFToken(h.tokenSecret, username, vote)},
		},
		Button: fmt.Sprintf("Vote for option %d", vote.Choice),
	}
	if err := confirmTmpl.Execute(w, data); err != nil {
		h.Errorf("failed to render vote confirmation: %s", err)
	}
}

func (h *HTTPSrv) recordVote(w http.ResponseWriter, r *http.Request, username string) {
	base.LimitRequestBody(w, r, formBodyLimit)
	if err := r.ParseForm(); err != nil {
		h.Debug("invalid vote form: %s", err)
		h.showError(w)
		return
	}
	vote, err := NewVoteFromEncoded(r.PostFormValue("vote"))
	if err != nil {
		h.Debug("invalid vote payload: %s", err)
		h.showError(w)
		return
	}
	expected := voteCSRFToken(h.tokenSecret, username, vote)
	if !hmac.Equal([]byte(r.PostFormValue("csrf")), []byte(expected)) {
		h.Debug("vote rejected: CSRF token mismatch")
		h.showError(w)
		return
	}
	// WithoutCancel: the browser submits the vote and may close the connection;
	// DB writes and poll result updates must complete regardless.
	ctx := context.WithoutCancel(r.Context())
	convID, resultMsgID, numChoices, err := h.db.GetPollInfo(ctx, vote.ID)
	if err != nil {
		h.Errorf("failed to find poll result msg: %s", err)
		h.showError(w)
		return
	}
	members, err := h.kbc.ListMembersByConvID(convID)
	if err != nil {
		h.Errorf("failed to list conv members: %s", err)
		h.showError(w)
		return
	}
	if !isConvMember(members, username) {
		h.Debug("vote rejected: %q is not a member of %q", username, convID)
		h.showError(w)
		return
	}
	if vote.Choice < 1 || vote.Choice > numChoices {
		h.Debug("vote choice %d out of range for poll %q", vote.Choice, vote.ID)
		h.showError(w)
		return
	}
	if err := h.db.CastVote(ctx, username, vote); err != nil {
		h.Errorf("failed to cast vote: %s", err)
		h.showError(w)
		return
	}
	tally, err := h.db.GetTally(ctx, vote.ID)
	if err != nil {
		h.Errorf("failed to get tally: %s", err)
		h.showError(w)
		return
	}
	if _, err := h.kbc.EditByConvID(convID, resultMsgID, formatTally(tally, numChoices)); err != nil {
		h.Errorf("failed to post result: %s", err)
		h.showError(w)
		return
	}
	h.showSuccess(w)
}

// handleLogin shows a confirmation page for a login link and sets the auth
// cookie only on POST, so a link alone can't log the browser into someone else's identity.
func (h *HTTPSrv) handleLogin(w http.ResponseWriter, r *http.Request) {
	base.SecureHeaders(w)
	switch r.Method {
	case http.MethodGet:
		nonce, err := base.RandBytes(16)
		if err != nil {
			h.Errorf("failed to make login nonce: %s", err)
			h.showError(w)
			return
		}
		csrf := hex.EncodeToString(nonce)
		// The POST must echo this cookie. A cross-site form can't read it, and a
		// SameSite=Strict cookie isn't sent on a cross-site POST, so the form fails.
		http.SetCookie(w, &http.Cookie{
			Name:     loginCSRFCookie,
			Value:    csrf,
			MaxAge:   int(loginConfirmMaxAge.Seconds()),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})
		username := r.URL.Query().Get("username")
		data := confirmData{
			Title:        "Login",
			Heading:      fmt.Sprintf("Log in as @%s?", username),
			Instructions: "Only continue if you requested this login yourself.",
			Action:       "/pollbot/login",
			Fields: []confirmField{
				{Name: "token", Value: r.URL.Query().Get("token")},
				{Name: "username", Value: username},
				{Name: "csrf", Value: csrf},
			},
			Button: "Log in",
		}
		if err := confirmTmpl.Execute(w, data); err != nil {
			h.Errorf("failed to render login confirmation: %s", err)
		}
	case http.MethodPost:
		h.completeLogin(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *HTTPSrv) completeLogin(w http.ResponseWriter, r *http.Request) {
	base.LimitRequestBody(w, r, formBodyLimit)
	if err := r.ParseForm(); err != nil {
		h.Debug("invalid login form: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	cookie, err := r.Cookie(loginCSRFCookie)
	if err != nil || !hmac.Equal([]byte(cookie.Value), []byte(r.PostFormValue("csrf"))) {
		h.Debug("login rejected: CSRF token mismatch")
		w.WriteHeader(http.StatusForbidden)
		return
	}
	token := r.PostFormValue("token")
	username := r.PostFormValue("username")
	if !base.VerifyLoginToken(h.tokenSecret, "pollbot-login", username, token) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "auth",
		Value:    fmt.Sprintf("%s:%s", username, token),
		Expires:  time.Now().Add(base.LoginTokenMaxAge),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	_, _ = w.Write([]byte(htmlLoginSuccess))
}

func (h *HTTPSrv) handleImage(w http.ResponseWriter, r *http.Request) {
	image := r.URL.Query().Get("")
	b64dat, ok := images[image]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	dat, _ := base64.StdEncoding.DecodeString(b64dat)
	if _, err := io.Copy(w, bytes.NewBuffer(dat)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (h *HTTPSrv) handleHealthCheck(_ http.ResponseWriter, _ *http.Request) {}

func isConvMember(members chat1.ChatMembersDetails, username string) bool {
	allMembers := [][]chat1.ChatMemberDetails{
		members.Owners, members.Admins, members.Writers, members.Readers, members.Bots,
	}
	for _, group := range allMembers {
		for _, m := range group {
			if m.Username == username {
				return true
			}
		}
	}
	return false
}
