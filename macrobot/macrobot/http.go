package macrobot

import (
	"net/http"

	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.HTTPSrv
}

func NewHTTPSrv(stats *base.StatsRegistry, debugConfig *base.ChatDebugOutputConfig) *HTTPSrv {
	h := &HTTPSrv{}
	h.HTTPSrv = base.NewHTTPSrv(stats, debugConfig)
	http.HandleFunc("/macrobot", h.handleHealthCheck)
	return h
}

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte("my life for aiur")); err != nil {
		h.Errorf("handleHealthCheck: unable to write: %v", err)
	}
}
