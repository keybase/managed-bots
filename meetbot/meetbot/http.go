package meetbot

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.OAuthHTTPSrv

	db      *base.OAuthDB
	handler *Handler
}

func NewHTTPSrv(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig,
	db *base.OAuthDB, handler *Handler, oauthConfig *oauth2.Config) *HTTPSrv {
	h := &HTTPSrv{
		db:      db,
		handler: handler,
	}
	h.OAuthHTTPSrv = base.NewOAuthHTTPSrv(stats, kbc, debugConfig, oauthConfig, h.db, h.handler.HandleAuth,
		"meetbot", base.Images["logo"], "/meetbot")
	http.HandleFunc("/meetbot", h.healthCheckHandler)
	http.HandleFunc("/meetbot/home", h.homeHandler)
	http.HandleFunc("/meetbot/image", h.handleImage)
	return h
}

func (h *HTTPSrv) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func (h *HTTPSrv) homeHandler(w http.ResponseWriter, r *http.Request) {
	h.Stats.Count("home")
	homePage := `Meetbot is a <a href="https://keybase.io"> Keybase</a> chatbot
	which creates links to Google Meet meetings for you.
	<div style="padding-top:25px;">
		<img style="width:300px;" src="/meetbot/image?=mobile">
	</div>
	`
	if _, err := w.Write(base.MakeOAuthHTML("meetbot", "home", homePage, "/meetbot/image?=logo")); err != nil {
		h.Errorf("homeHandler: unable to write: %v", err)
	}
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
