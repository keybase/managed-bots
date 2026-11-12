package base

import (
	"context"
	"net/http"
	"time"
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
		srv: &http.Server{
			Addr:              ":8080",
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}
}

func (h *HTTPSrv) Listen() (err error) {
	defer h.Trace(&err, "ListenAndServe")()
	return h.srv.ListenAndServe()
}

func (h *HTTPSrv) Shutdown() (err error) {
	defer h.Trace(&err, "Shutdown")()
	return h.srv.Shutdown(context.Background())
}

// LimitRequestBody caps how much of the request body a handler will read.
// Call it before reading r.Body; reads past limit fail with an error.
func LimitRequestBody(w http.ResponseWriter, r *http.Request, limit int64) {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
}

// SecureHeaders sets anti-framing, referrer and sniffing headers on a response.
// Use it on every HTML page a bot serves.
func SecureHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("X-Frame-Options", "DENY")
	h.Set("Content-Security-Policy", "frame-ancestors 'none'")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Content-Type-Options", "nosniff")
}

// SecureHandler wraps an HTML page handler so every response gets SecureHeaders.
// Wrap the route instead of calling SecureHeaders in each handler.
func SecureHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		SecureHeaders(w)
		next(w, r)
	}
}
