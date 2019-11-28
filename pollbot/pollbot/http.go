package pollbot

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

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
	w.Write([]byte(fmt.Sprint(`
		<html>
		<head>
			<title>Polling Service Confirmation</title>
		</head>
		<body>
			<h1>Login Required</h1>
			In order to vote in anonymous polls, you must first login into the polling service in your web browser. To do this, message @pollbot on Keybase with the text "!login". This can be done by just starting a new conversation with @pollbot and sending it "!login", or typing "/msg pollbot !login" in the chat input box. 
		</body>
		</html>
	`)))
}

func (h *HTTPSrv) showSuccess(w http.ResponseWriter) {
	w.Write([]byte(fmt.Sprint(`
		<html>
		<head>
			<title>Polling Service Confirmation</title>
		</head>
		<body>
			<h1>Success!</h1>
		</body>
		</html>
	`)))
}

func (h *HTTPSrv) showError(w http.ResponseWriter, msg string) {
	w.Write([]byte(fmt.Sprint(`
		<html>
		<head>
			<title>Polling Service Confirmation</title>
		</head>
		<body>
			<h1>Vote Failed</h1>
			%s
		</body>
		</html>
	`, msg)))
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
	if realToken != token {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.Header().Add("Set-Cookie", fmt.Sprintf("auth=%s:%s", username, token))
	w.Write([]byte(fmt.Sprint(`
		<html>
		<head>
			<title>Polling Service Confirmation</title>
		</head>
		<body>
			<h1>Login Success!</h1>
			You can now vote in anonymous polls by hitting links from @pollbot in the Keybase app.
		</body>
		</html>
	`)))
}

func (h *HTTPSrv) Listen() {
	http.HandleFunc("/pollbot/vote", h.handleVote)
	http.HandleFunc("/pollbot/login", h.handleLogin)
	http.ListenAndServe(":8080", nil)
}

func (h *HTTPSrv) LoginToken(username string) string {
	return hex.EncodeToString(hmac.New(sha256.New, []byte(h.tokenSecret)).Sum([]byte(username)))
}
