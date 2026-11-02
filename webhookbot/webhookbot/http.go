package webhookbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/didip/tollbooth/v8"
	"github.com/gorilla/mux"
	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.HTTPSrv

	db *DB
}

func NewHTTPSrv(stats *base.StatsRegistry, debugConfig *base.ChatDebugOutputConfig, db *DB) *HTTPSrv {
	h := &HTTPSrv{
		db: db,
	}
	h.HTTPSrv = base.NewHTTPSrv(stats, debugConfig)
	rtr := mux.NewRouter()
	rtr.HandleFunc("/webhookbot", h.handleHealthCheck)
	// restrict to 1req/sec
	rtr.Handle("/webhookbot/{id:[A-Za-z0-9_-]+}", tollbooth.LimitFuncHandler(tollbooth.NewLimiter(1, nil), h.handleHook))
	http.Handle("/", rtr)
	return h
}

type msgPayload struct {
	Msg string
}

func (h *HTTPSrv) getMessage(r *http.Request) (string, error) {
	msg := r.URL.Query().Get("msg")
	if len(msg) > 0 {
		return msg, nil
	}

	defer func() {
		if cerr := r.Body.Close(); cerr != nil {
			h.Errorf("getMessage: failed to close request body: %s", cerr)
		}
	}()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	var payload msgPayload
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&payload); err == nil && len(payload.Msg) > 0 {
		return payload.Msg, nil
	}

	msg = string(body)
	if len(msg) > 0 {
		return msg, nil
	}
	return "`Error: no body found. To use a webhook URL, supply a 'msg' URL parameter, or a JSON POST body with a field 'msg'`", nil
}

func (h *HTTPSrv) handleHook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	// WithoutCancel: the caller may close the connection immediately after
	// sending the webhook payload; DB reads and Keybase message sends must
	// complete regardless.
	ctx := context.WithoutCancel(r.Context())
	hook, err := h.db.GetHook(ctx, id)
	if err != nil {
		h.Stats.Count("handle - not found")
		h.Debug("handleHook: failed to find hook for ID: %s", id)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	msg, err := h.getMessage(r)
	if err != nil {
		h.Stats.Count("handle - no message")
		h.Errorf("handleHook: failed to find message: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	h.Stats.Count("handle - success")
	title := fmt.Sprintf("[hook: *%s*]", hook.Name)
	if err := h.SendMessageByConvIDOrFile(hook.ConvID, title, "%s\n\n%s", title, msg); err != nil {
		h.Debug("handleHook: failed to send message: %s", err)
	}
}

func (h *HTTPSrv) handleHealthCheck(_ http.ResponseWriter, _ *http.Request) {}
