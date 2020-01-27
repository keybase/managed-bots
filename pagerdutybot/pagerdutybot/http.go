package pagerdutybot

import (
	"net/http"

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

}

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, r *http.Request) {}
