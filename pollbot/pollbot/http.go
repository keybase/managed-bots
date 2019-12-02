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
)

type HTTPSrv struct {
	kbc *kbchat.API
	db  *DB

	tokenSecret string
}

func NewHTTPSrv(kbc *kbchat.API, db *DB, tokenSecret string) *HTTPSrv {
	return &HTTPSrv{
		kbc:         kbc,
		db:          db,
		tokenSecret: tokenSecret,
	}
}

func (h *HTTPSrv) debug(msg string, args ...interface{}) {
	fmt.Printf("HTTPSrv: "+msg+"\n", args...)
}

func (h *HTTPSrv) showLoginInstructions(w http.ResponseWriter) {
	w.Write([]byte(htmlLogin))
}

func (h *HTTPSrv) showSuccess(w http.ResponseWriter) {
	w.Write([]byte(makeHTMLVoteResult("Vote success!")))
}

func (h *HTTPSrv) showError(w http.ResponseWriter, msg string) {
	w.Write([]byte(makeHTMLVoteResult("Something went wrong, vote not recorded.")))
}

func (h *HTTPSrv) checkLogin(w http.ResponseWriter, r *http.Request) (string, bool) {
	cookie, err := r.Cookie("auth")
	if err != nil {
		h.debug("error getting cookie: %s", err)
		h.showLoginInstructions(w)
		return "", false
	}
	if cookie == nil {
		h.debug("no cookie")
		h.showLoginInstructions(w)
		return "", false
	}
	auth := cookie.Value
	toks := strings.Split(auth, ":")
	if len(toks) != 2 {
		h.debug("malformed auth cookie")
		h.showLoginInstructions(w)
		return "", false
	}
	username := toks[0]
	token := toks[1]
	if !hmac.Equal([]byte(token), []byte(h.LoginToken(username))) {
		h.debug("invalid auth cookie")
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
		h.debug("failed to cast vote: %s", err)
		h.showError(w, err.Error())
		return
	}
	resultMsgID, err := h.db.GetPollResultMsgID(vote.ConvID, vote.MsgID)
	if err != nil {
		h.debug("failed to find poll result msg: %s", err)
		h.showError(w, err.Error())
		return
	}
	tally, err := h.db.GetTally(vote.ConvID, vote.MsgID)
	if err != nil {
		h.debug("failed to get tally: %s", err)
		h.showError(w, err.Error())
		return
	}
	if _, err := h.kbc.EditByConvID(vote.ConvID, resultMsgID, formatTally(tally)); err != nil {
		h.debug("failed to post result: %s", err)
		h.showError(w, err.Error())
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
	w.Write([]byte(htmlLoginSuccess))
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

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, r *http.Request) {}

func (h *HTTPSrv) Listen() error {
	http.HandleFunc("/pollbot", h.handleHealthCheck)
	http.HandleFunc("/pollbot/vote", h.handleVote)
	http.HandleFunc("/pollbot/login", h.handleLogin)
	http.HandleFunc("/pollbot/image", h.handleImage)
	return http.ListenAndServe(":8080", nil)
}

func (h *HTTPSrv) LoginToken(username string) string {
	return hex.EncodeToString(hmac.New(sha256.New, []byte(h.tokenSecret)).Sum([]byte(username)))
}
