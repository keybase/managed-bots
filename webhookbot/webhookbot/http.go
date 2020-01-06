package webhookbot

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.HTTPSrv

	kbc *kbchat.API
	db  *DB
}

func NewHTTPSrv(kbc *kbchat.API, db *DB) *HTTPSrv {
	h := &HTTPSrv{
		kbc: kbc,
		db:  db,
	}
	h.HTTPSrv = base.NewHTTPSrv(kbc)
	rtr := mux.NewRouter()
	rtr.HandleFunc("/webhookbot", h.handleHealthCheck)
	rtr.HandleFunc("/webhookbot/{id:[A-Za-z0-9_]+}", h.handleHook)
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

	var payload msgPayload
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&payload); err != nil {
		return msg, err
	}
	return payload.Msg, nil
}

func (h *HTTPSrv) handleHook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	hook, err := h.db.GetHook(id)
	if err != nil {
		h.Debug("handleHook: failed to find hook for ID: %s", id)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	msg, err := h.getMessage(r)
	if err != nil {
		h.Debug("handleHook: failed to find message: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	h.ChatEcho(hook.convID, "[hook: *%s*]\n\n%s", hook.name, msg)
}

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, r *http.Request) {}
