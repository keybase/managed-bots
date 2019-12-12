package webhookbot

import (
	"net/http"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
)

type HTTPSrv struct {
	kbc *kbchat.API
	db  *DB
}

func NewHTTPSrv(kbc *kbchat.API, db *DB) *HTTPSrv {
	return &HTTPSrv{
		kbc: kbc,
		db:  db,
	}
}

func (h *HTTPSrv) handleHook(w http.ResponseWriter, r *http.Request) {

}

func (h *HTTPSrv) Listen() error {
	http.HandleFunc("/webhookbot", h.handleHook)
	return http.ListenAndServe(":8080", nil)
}
