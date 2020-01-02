package base

import (
	"context"
	"net/http"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
)

type HTTPSrv struct {
	*DebugOutput
	srv *http.Server
}

func NewHTTPSrv(kbc *kbchat.API) *HTTPSrv {
	return &HTTPSrv{
		DebugOutput: NewDebugOutput("HTTPSrv", kbc),
		srv:         &http.Server{Addr: ":8080"},
	}
}

func (h *HTTPSrv) Listen() error {
	return h.srv.ListenAndServe()
}

func (h *HTTPSrv) Shutdown() error {
	return h.srv.Shutdown(context.Background())
}
