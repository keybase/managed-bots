package webhookbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/gorilla/mux"
	"github.com/keybase/managed-bots/base"
)

type HTTPSrv struct {
	*base.HTTPSrv

	db *DB
}

func NewHTTPSrv(stats *base.StatsRegistry, debugConfig *base.ChatDebugOutputConfig, db *DB) *HTTPSrv {
	h := &HTTPSrv{
		db: db,
	}
	h.HTTPSrv = base.NewHTTPSrv(stats, debugConfig)
	rtr := mux.NewRouter()
	rtr.HandleFunc("/webhookbot", h.handleHealthCheck)
	// restrict to 1req/sec
	rtr.Handle("/webhookbot/{id:[A-Za-z0-9_-]+}", tollbooth.LimitFuncHandler(tollbooth.NewLimiter(1, nil), h.handleHook))
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

	defer r.Body.Close()
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
	hook, err := h.db.GetHook(id)
	if err != nil {
		h.Stats.Count("handle - not found")
		h.Debug("handleHook: failed to find hook for ID: %s", id)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	msg, err := h.getMessage(r)
	if err != nil {
		h.Stats.Count("handle - no message")
		h.Errorf("handleHook: failed to find message: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	h.Stats.Count("handle - success")
	if _, err := h.Config().KBC.SendMessageByConvID(hook.convID, "[hook: *%s*]\n\n%s", hook.name, msg); err != nil {
		if base.IsDeletedConvError(err) {
			h.Debug("ChatEcho: failed to send echo message: %s", err)
			return
		}

		// error created in https://github.com/keybase/client/blob/7d6aa64f3fba66adba7a5dd1cc7c523d5086a548/go/chat/msgchecker/plaintext_checker.go#L50
		if strings.Contains(err.Error(), "exceeds the maximum length") {
			fileName := fmt.Sprintf("webhookbot-%s-%d.txt", hook.name, time.Now().Unix())
			filePath := fmt.Sprintf("/tmp/%s", fileName)
			if err := os.WriteFile(filePath, []byte(msg), 0644); err != nil {
				h.Errorf("failed to write %s: %s", filePath, err)
				return
			}
			base.GoWithRecover(h.DebugOutput, func() {
				defer func() {
					// Cleanup after the file is sent.
					time.Sleep(time.Minute)
					h.Debug("cleaning up %s", filePath)
					if err = os.Remove(filePath); err != nil {
						h.Errorf("unable to clean up %s: %v", filePath, err)
					}
				}()
				title := fmt.Sprintf("[hook: *%s*]", hook.name)
				if _, err := h.Config().KBC.SendAttachmentByConvID(hook.convID, filePath, title); err != nil {
					h.Errorf("failed to send attachment %s: %s", filePath, err)
					return
				}
			})
			return
		}

		h.Errorf("ChatEcho: failed to send echo message: %s", err)
	}
}

func (h *HTTPSrv) handleHealthCheck(w http.ResponseWriter, r *http.Request) {}
