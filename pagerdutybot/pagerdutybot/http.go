package pagerdutybot

import (
	"net/http"

	pd "github.com/PagerDuty/go-pagerduty"
	"github.com/gorilla/mux"
	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.HTTPSrv

	db *DB
}

func NewHTTPSrv(debugConfig *base.ChatDebugOutputConfig, db *DB) *HTTPSrv {
	h := &HTTPSrv{
		db: db,
	}
	h.HTTPSrv = base.NewHTTPSrv(debugConfig)
	rtr := mux.NewRouter()
	rtr.HandleFunc("/pagerdutybot", h.handleHealthCheck)
	rtr.HandleFunc("/pagerdutybot/{id:[A-Za-z0-9_]+}", h.handleHook)
	http.Handle("/", rtr)
	return h
}

func (h *HTTPSrv) handleHook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	convID, err := h.db.GetHook(id)
	if err != nil {
		h.Debug("handleHook: failed to find hook for ID: %s", id)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	payload, err := pd.DecodeWebhook(r.Body)
	if err != nil {
		h.Debug("handleHook: failed to decode: %s", err)
		w.WriteHeader(http.StatusNotFound)
	}
	h.ChatEcho(convID, "%s", payload.Type)
}

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, r *http.Request) {}
