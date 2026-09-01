package pollbot

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
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
	if !base.VerifyLoginToken(h.tokenSecret, username, token) {
		h.Debug("invalid auth cookie")
		h.showLoginInstructions(w)
		return "", false
	}
	return username, true
}

func (h *HTTPSrv) handleVote(w http.ResponseWriter, r *http.Request) {
	username, ok := h.checkLogin(w, r)
	if !ok {
		return
	}
	vstr := r.URL.Query().Get("")
	vote, err := NewVoteFromEncoded(vstr)
	if err != nil {
		h.Debug("invalid vote payload: %s", err)
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

func (h *HTTPSrv) handleLogin(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	username := r.URL.Query().Get("username")
	if !base.VerifyLoginToken(h.tokenSecret, username, token) {
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
