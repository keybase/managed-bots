package webhookbot

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.HTTPSrv

	db    *DB
	stats *base.StatsRegistry
}

func NewHTTPSrv(stats *base.StatsRegistry, debugConfig *base.ChatDebugOutputConfig, db *DB) *HTTPSrv {
	h := &HTTPSrv{
		db: db,
	}
	h.HTTPSrv = base.NewHTTPSrv(stats, debugConfig)
	rtr := mux.NewRouter()
	rtr.HandleFunc("/webhookbot", h.handleHealthCheck)
	rtr.HandleFunc("/webhookbot/{id:[A-Za-z0-9_-]+}", h.handleHook)
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

	var buf bytes.Buffer
	bodyTee := io.TeeReader(r.Body, &buf)
	var payload msgPayload
	decoder := json.NewDecoder(bufio.NewReader(bodyTee))
	if err := decoder.Decode(&payload); err != nil {
		return "", err
	} else if len(payload.Msg) > 0 {
		return payload.Msg, nil
	}

	body, err := ioutil.ReadAll(&buf)
	if err != nil {
		return "", err
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
	hook, err := h.db.GetHook(id)
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
	h.ChatEcho(hook.convID, "[hook: *%s*]\n\n%s", hook.name, msg)
}

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, r *http.Request) {}
