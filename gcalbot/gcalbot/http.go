package gcalbot

import (
	"fmt"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.OAuthHTTPSrv

	kbc             *kbchat.API
	db              *DB
	handler         *Handler
	webhookChannels *WebhookChannels
}

func NewHTTPSrv(
	kbc *kbchat.API,
	db *DB,
	handler *Handler,
	requests *base.OAuthRequests,
	webhookChannels *WebhookChannels,
	config *oauth2.Config,
) *HTTPSrv {
	h := &HTTPSrv{
		kbc:             kbc,
		db:              db,
		handler:         handler,
		webhookChannels: webhookChannels,
	}
	h.OAuthHTTPSrv = base.NewOAuthHTTPSrv(kbc, config, requests, h.db, h.handler.HandleAuth,
		"gcalbot", base.Images["logo"], "/gcalbot")
	http.HandleFunc("/gcalbot", h.healthCheckHandler)
	http.HandleFunc("/gcalbot/events/webhook", h.handleEventUpdateWebhook)
	return h
}

func (h *HTTPSrv) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}
