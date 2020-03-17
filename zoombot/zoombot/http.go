package zoombot

import (
	"fmt"
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
		"zoombot", base.Images["logo"], "/zoombot")
	http.HandleFunc("/zoombot", h.healthCheckHandler)
	return h
}

func (h *HTTPSrv) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}
