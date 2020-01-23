package base

import (
	"context"
	"net/http"
)

type HTTPSrv struct {
	*DebugOutput
	srv *http.Server
}

func NewHTTPSrv(debugConfig *ChatDebugOutputConfig) *HTTPSrv {
	return &HTTPSrv{
		DebugOutput: NewDebugOutput("HTTPSrv", debugConfig),
		srv:         &http.Server{Addr: ":8080"},
	}
}

func (h *HTTPSrv) Listen() error {
	return h.srv.ListenAndServe()
}

func (h *HTTPSrv) Shutdown() error {
	return h.srv.Shutdown(context.Background())
}
