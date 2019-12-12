package webhookbot

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.DebugOutput

	kbc *kbchat.API
	db  *DB
}

func NewHTTPSrv(kbc *kbchat.API, db *DB) *HTTPSrv {
	return &HTTPSrv{
		DebugOutput: base.NewDebugOutput("HTTPSrv", kbc),
		kbc:         kbc,
		db:          db,
	}
}

func (h *HTTPSrv) getMessage(r *http.Request) (string, error) {
	msg := r.URL.Query().Get("msg")
	if len(msg) == 0 {
		return "", errors.New("no message given")
	}
	return msg, nil
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

func (h *HTTPSrv) Listen() error {
	rtr := mux.NewRouter()
	rtr.HandleFunc("/webhookbot", h.handleHealthCheck)
	rtr.HandleFunc("/webhookbot/{id:[A-Za-z0-9]+}", h.handleHook)
	http.Handle("/", rtr)
	return http.ListenAndServe(":8080", nil)
}
