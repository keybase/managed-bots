package zoombot

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

func (h *HTTPSrv) healthCheckHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "OK")
}

// see https://developers.zoom.us/docs/api/webhooks/#verify-with-zooms-header
func (h *HTTPSrv) validateWebhookMessage(bodyBytes []byte, r *http.Request) (err error) {
	requestTimestamp := r.Header.Get("x-zm-request-timestamp")
	message := fmt.Sprintf("v0:%s:%s", requestTimestamp, bodyBytes)
	hash := hmac.New(sha256.New, []byte(h.credentials.SecretToken))
	_, err = hash.Write([]byte(message))
	if err != nil {
		return err
	}
	actualSig := fmt.Sprintf("v0=%s", hex.EncodeToString(hash.Sum(nil)))
	givenSig := r.Header.Get("x-zm-signature")
	if givenSig != actualSig {
		return fmt.Errorf("invalid signature %s!=%s", actualSig, givenSig)
	}
	return nil
}

func (h *HTTPSrv) zoomDeauthorize(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		h.Errorf("zoomDeauthorize: unable to read body: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = h.validateWebhookMessage(bodyBytes, r)
	if err != nil {
		h.Errorf("zoomDeauthorize: invalid signature: %s", err)
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	reader := bytes.NewReader(bodyBytes)
	var deauthorizationRequest DeauthorizationRequest
	err = json.NewDecoder(reader).Decode(&deauthorizationRequest)
	if err != nil {
		h.Errorf("zoomDeauthorize: parsing error: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
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
