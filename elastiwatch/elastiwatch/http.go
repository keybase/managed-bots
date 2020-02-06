package elastiwatch

import (
	"net/http"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.HTTPSrv

	kbc *kbchat.API
	db  *DB
}

func NewHTTPSrv(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig, db *DB) *HTTPSrv {
	h := &HTTPSrv{
		HTTPSrv: base.NewHTTPSrv(stats, debugConfig),
		kbc:     kbc,
		db:      db,
	}
	http.HandleFunc("/elastiwatch", h.handleHealthCheck)
	return h
}

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, r *http.Request) {}
