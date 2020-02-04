package base

import (
	"context"
	"net/http"
)

type HTTPSrv struct {
	*DebugOutput
	srv   *http.Server
	Stats *StatsRegistry
}

func NewHTTPSrv(stats *StatsRegistry, debugConfig *ChatDebugOutputConfig) *HTTPSrv {
	return &HTTPSrv{
		DebugOutput: NewDebugOutput("HTTPSrv", debugConfig),
		Stats:       stats.SetPrefix("HTTPSrv"),
		srv:         &http.Server{Addr: ":8080"},
	}
}

func (h *HTTPSrv) Listen() error {
	return h.srv.ListenAndServe()
}

func (h *HTTPSrv) Shutdown() error {
	return h.srv.Shutdown(context.Background())
}
