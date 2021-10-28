package webhookbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

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
	rtr.HandleFunc("/webhookbot/{id:[A-Za-z0-9_-]+}", h.handleHook)
	http.Handle("/", rtr)
	return h
}

func injectTemplateVars(webhookName, webhookMethod, template string) string {
	return fmt.Sprintf("{{$webhookName := %q}}{{$webhookMethod := %q}} %s", webhookName, webhookMethod, template)
}

func (h *HTTPSrv) getMessage(r *http.Request, hook webhook) (string, error) {
	if r.Method == http.MethodGet && len(hook.template) == 0 {
		j, err := json.Marshal(r.URL.Query())
		if err != nil {
			return "", err
		}
		return string(j), nil
	}

	if r.Method == http.MethodPost && len(hook.template) == 0 {
		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}

	tWithVars := injectTemplateVars(hook.name, r.Method, hook.template)
	t, err := template.New("").Parse(tWithVars)
	if err != nil {
		return "`Error: failed to parse template: " + err.Error() + "`", nil
	}

	if r.Method == http.MethodGet {
		buf := new(bytes.Buffer)
		if err := t.Execute(buf, r.URL.Query()); err == nil {
			return buf.String(), nil
		}
	}

	if r.Method == http.MethodPost {
		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)

		m := map[string]interface{}{}
		err = json.Unmarshal(body, &m)
		if err != nil {
			return "", err
		}
		buf := new(bytes.Buffer)
		if err := t.Execute(buf, m); err == nil && len(buf.String()) > 0 {
			return buf.String(), nil
		}
	}
	return "", nil
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
	msg, err := h.getMessage(r, hook)
	if err != nil {
		h.Stats.Count("handle - no message")
		h.Errorf("handleHook: failed to find message: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(msg) == "" {
		h.Stats.Count("handle - empty msg")
		return
	}
	h.Stats.Count("handle - success")
	if _, err := h.Config().KBC.SendMessageByConvID(hook.convID, " %s", msg); err != nil {
		if base.IsDeletedConvError(err) {
			h.Debug("ChatEcho: failed to send echo message: %s", err)
			return
		}

		// error created in https://github.com/keybase/client/blob/7d6aa64f3fba66adba7a5dd1cc7c523d5086a548/go/chat/msgchecker/plaintext_checker.go#L50
		if strings.Contains(err.Error(), "exceeds the maximum length") {
			fileName := fmt.Sprintf("webhookbot-%s-%d.txt", hook.name, time.Now().Unix())
			filePath := fmt.Sprintf("/tmp/%s", fileName)
			if err := ioutil.WriteFile(filePath, []byte(msg), 0644); err != nil {
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
