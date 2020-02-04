package gcalbot

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

	db      *DB
	handler *Handler

	reminderScheduler ReminderScheduler
}

func NewHTTPSrv(
	stats *base.StatsRegistry,
	kbc *kbchat.API,
	debugConfig *base.ChatDebugOutputConfig,
	db *DB,
	oauthConfig *oauth2.Config,
	reminderScheduler ReminderScheduler,
	handler *Handler,
) *HTTPSrv {
	h := &HTTPSrv{
		db:                db,
		handler:           handler,
		reminderScheduler: reminderScheduler,
	}
	h.OAuthHTTPSrv = base.NewOAuthHTTPSrv(stats, kbc, debugConfig, oauthConfig, h.db, h.handler.HandleAuth,
		"gcalbot", base.Images["logo"], "/gcalbot")
	http.HandleFunc("/gcalbot", h.healthCheckHandler)
	http.HandleFunc("/gcalbot/home", h.homeHandler)
	http.HandleFunc("/gcalbot/image/screenshot", h.screenshotHandler)
	http.HandleFunc("/gcalbot/events/webhook", h.handleEventUpdateWebhook)
	return h
}

func (h *HTTPSrv) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprintf(w, "OK")
}

func (h *HTTPSrv) homeHandler(w http.ResponseWriter, r *http.Request) {
	h.Stats.Count("home")
	homePage := `Google Calendar Bot is a <a href="https://keybase.io">Keybase</a> chatbot
	which connects with your Google calendar to notify you of invites, upcoming events and more!
	<div style="padding-top:25px;">
		<img style="width:300px;" src="/gcalbot/image/screenshot">
	</div>
	`
	if _, err := w.Write(base.MakeOAuthHTML("gcalbot", "home", homePage, "/gcalbot/image/logo")); err != nil {
		h.Errorf("homeHandler: unable to write: %v", err)
	}
}

func (h *HTTPSrv) screenshotHandler(w http.ResponseWriter, r *http.Request) {
	dat, _ := base64.StdEncoding.DecodeString(screenshot)
	if _, err := io.Copy(w, bytes.NewBuffer(dat)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
