package zoombot

import (
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.OAuthHTTPSrv

	db      *DB
	handler *Handler

	credentials *Credentials
}

func NewHTTPSrv(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig,
	db *DB, handler *Handler, oauthConfig *oauth2.Config, credentials *Credentials) *HTTPSrv {
	h := &HTTPSrv{
		db:          db,
		handler:     handler,
		credentials: credentials,
	}
	h.OAuthHTTPSrv = base.NewOAuthHTTPSrv(stats, kbc, debugConfig, oauthConfig, h.db, h.handler.HandleAuth,
		"zoombot", base.Images["logo"], "/zoombot")
	http.HandleFunc("/zoombot", h.healthCheckHandler)
	http.HandleFunc("/zoombot/deauthorize", h.zoomDeauthorize)
	return h
}

func (h *HTTPSrv) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "OK")
}

func (h *HTTPSrv) zoomDeauthorize(w http.ResponseWriter, r *http.Request) {
	var deauthorizationRequest DeauthorizationRequest
	err := json.NewDecoder(r.Body).Decode(&deauthorizationRequest)
	if err != nil {
		h.Errorf("zoomDeauthorize: parsing error: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	verificationToken := r.Header.Get("Authorization")
	if verificationToken != h.credentials.VerificationToken {
		h.Errorf("zoomDeauthorize: wrong verificationToken: %s", verificationToken)
		http.Error(w, "wrong verificationToken", http.StatusBadRequest)
		return
	}

	if deauthorizationRequest.Payload.ClientID != h.credentials.ClientID {
		h.Errorf("zoomDeauthorize: wrong clientID: %s", deauthorizationRequest.Payload.ClientID)
		http.Error(w, "wrong clientID", http.StatusBadRequest)
		return
	}

	err = h.db.DeleteUserAndToken(deauthorizationRequest.Payload.UserID, deauthorizationRequest.Payload.AccountID)
	if err != nil {
		h.Errorf("zoomDeauthorize: unable to delete user: %s", err)
		http.Error(w, "unable to delete user", http.StatusBadRequest)
		return
	}

	_, err = DataCompliance(h.credentials.ClientID, h.credentials.ClientSecret, &DataComplianceRequest{
		ClientID:                     deauthorizationRequest.Payload.ClientID,
		UserID:                       deauthorizationRequest.Payload.UserID,
		AccountID:                    deauthorizationRequest.Payload.AccountID,
		DeauthorizationEventReceived: deauthorizationRequest.Payload,
		ComplianceCompleted:          true,
	})
	if err != nil {
		h.Errorf("zoomDeauthorize: compliance error: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
