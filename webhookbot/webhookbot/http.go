package webhookbot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/didip/tollbooth/v8"
	"github.com/didip/tollbooth/v8/limiter"
	"github.com/gorilla/mux"
	"github.com/keybase/managed-bots/base"
)

// webhookBodyLimit caps a hook's message body. Larger messages are not useful
// as chat text; the attachment fallback in SendMessageByConvIDOrFile is only
// reached for messages under this limit.
const webhookBodyLimit = 1 << 20

// newRateLimiter allows one request per second per key. Idle keys expire after
// an hour so the limiter's memory tracks recent traffic only.
func newRateLimiter() *limiter.Limiter {
	return tollbooth.NewLimiter(1, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
}

// newIPLimiter limits each client IP. bots.keybase.io is an Application Load
// Balancer, which appends the connecting client's address to X-Forwarded-For,
// so the rightmost entry is the client and can't be spoofed by the sender.
// RemoteAddr would be the load balancer itself, one bucket for every client.
func newIPLimiter() *limiter.Limiter {
	return newRateLimiter().SetIPLookup(limiter.IPLookup{Name: "X-Forwarded-For", IndexFromRight: 0})
}

type HTTPSrv struct {
	*base.HTTPSrv

	db *DB
	// hookLimiter is keyed by hook ID, so a hook's limit doesn't depend on
	// which address the request appears to come from.
	hookLimiter *limiter.Limiter
}

func NewHTTPSrv(stats *base.StatsRegistry, debugConfig *base.ChatDebugOutputConfig, db *DB) *HTTPSrv {
	h := &HTTPSrv{
		db:          db,
		hookLimiter: newRateLimiter(),
	}
	h.HTTPSrv = base.NewHTTPSrv(stats, debugConfig)
	rtr := mux.NewRouter()
	rtr.HandleFunc("/webhookbot", h.handleHealthCheck)
	rtr.Handle("/webhookbot/{id:[A-Za-z0-9_-]+}", tollbooth.LimitFuncHandler(newIPLimiter(), h.handleHook))
	http.Handle("/", rtr)
	return h
}

type msgPayload struct {
	Msg string
}

func (h *HTTPSrv) getMessage(r *http.Request) (string, error) {
	msg := r.URL.Query().Get("msg")
	if len(msg) > 0 {
		return msg, nil
	}

	defer func() {
		if cerr := r.Body.Close(); cerr != nil {
			h.Errorf("getMessage: failed to close request body: %s", cerr)
		}
	}()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	var payload msgPayload
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&payload); err == nil && len(payload.Msg) > 0 {
		return payload.Msg, nil
	}

	msg = string(body)
	if len(msg) > 0 {
		return msg, nil
	}
	return "`Error: no body found. To use a webhook URL, supply a 'msg' URL parameter, or a JSON POST body with a field 'msg'`", nil
}

func (h *HTTPSrv) handleHook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	base.LimitRequestBody(w, r, webhookBodyLimit)
	// WithoutCancel: the caller may close the connection immediately after
	// sending the webhook payload; DB reads and Keybase message sends must
	// complete regardless.
	ctx := context.WithoutCancel(r.Context())
	hook, err := h.db.GetHook(ctx, id)
	if err != nil {
		h.Stats.Count("handle - not found")
		h.Debug("handleHook: failed to find hook for ID: %s", id)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err := tollbooth.LimitByKeys(h.hookLimiter, []string{id}); err != nil {
		h.Stats.Count("handle - rate limited")
		http.Error(w, err.Message, err.StatusCode)
		return
	}
	msg, err := h.getMessage(r)
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			h.Stats.Count("handle - too large")
			http.Error(w, "webhook payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		h.Stats.Count("handle - no message")
		h.Errorf("handleHook: failed to find message: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	h.Stats.Count("handle - success")
	title := fmt.Sprintf("[hook: *%s*]", hook.Name)
	if err := h.SendMessageByConvIDOrFile(hook.ConvID, title, "%s\n\n%s", title, msg); err != nil {
		h.Debug("handleHook: failed to send message: %s", err)
	}
}

func (h *HTTPSrv) handleHealthCheck(_ http.ResponseWriter, _ *http.Request) {}
