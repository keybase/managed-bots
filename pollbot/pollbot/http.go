package pollbot

import (
	"bytes"
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
	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.HTTPSrv

	kbc *kbchat.API
	db  *DB

	tokenSecret string
}

func NewHTTPSrv(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig,
	db *DB, tokenSecret string) *HTTPSrv {
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
	toks := strings.Split(auth, ":")
	if len(toks) != 2 {
		h.Debug("malformed auth cookie %v", auth)
		h.showLoginInstructions(w)
		return "", false
	}
	username := toks[0]
	token := toks[1]
	if !hmac.Equal([]byte(token), []byte(h.LoginToken(username))) {
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
	vote := NewVoteFromEncoded(vstr)
	if err := h.db.CastVote(username, vote); err != nil {
		h.Errorf("failed to cast vote: %s", err)
		h.showError(w)
		return
	}
	convID, resultMsgID, numChoices, err := h.db.GetPollInfo(vote.ID)
	if err != nil {
		h.Errorf("failed to find poll result msg: %s", err)
		h.showError(w)
		return
	}
	tally, err := h.db.GetTally(vote.ID)
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
	realToken := h.LoginToken(username)
	if !hmac.Equal([]byte(realToken), []byte(token)) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:    "auth",
		Value:   fmt.Sprintf("%s:%s", username, token),
		Expires: time.Now().Add(8760 * time.Hour),
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

func (h *HTTPSrv) LoginToken(username string) string {
	return hex.EncodeToString(hmac.New(sha256.New, []byte(h.tokenSecret)).Sum([]byte(username)))
}
