package webhookbot

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"text/template"

	_ "github.com/go-sql-driver/mysql"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

const defaultTemplate = "[hook: *{{$webhookName}}*]\n\n{{if eq $webhookMethod \"POST\"}}{{.msg}}{{else}}{{index .msg 0}}{{end}}"

type Handler struct {
	*base.DebugOutput

	stats      *base.StatsRegistry
	kbc        *kbchat.API
	db         *DB
	httpSrv    *HTTPSrv
	httpPrefix string
}

var _ base.Handler = (*Handler)(nil)

func NewHandler(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig,
	httpSrv *HTTPSrv, db *DB, httpPrefix string) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		stats:       stats.SetPrefix("Handler"),
		kbc:         kbc,
		db:          db,
		httpSrv:     httpSrv,
		httpPrefix:  httpPrefix,
	}
}

func (h *Handler) formURL(id string) string {
	return fmt.Sprintf("%s/webhookbot/%s", h.httpPrefix, id)
}

var errNotAllowed = errors.New("must be at least a writer to administer webhooks")

func (h *Handler) checkAllowed(msg chat1.MsgSummary) error {
	ok, err := base.IsAtLeastWriter(h.kbc, msg.Sender.Username, msg.Channel)
	if err != nil {
		return fmt.Errorf("handleCreate: failed to check role: %s", err)
	}
	if !ok {
		return errNotAllowed
	}
	return nil
}

func (h *Handler) handleRemove(cmd string, msg chat1.MsgSummary) (err error) {
	convID := msg.ConvID
	toks := strings.Fields(cmd)
	if len(toks) != 3 {
		h.ChatEcho(convID, "invalid number of arguments, must specify a name")
		return nil
	}
	err = h.checkAllowed(msg)
	switch err {
	case nil:
	case errNotAllowed:
		h.ChatEcho(convID, err.Error())
		return nil
	default:
		return err
	}
	h.stats.Count("remove")
	name := toks[2]
	if err := h.db.Remove(name, convID); err != nil {
		return fmt.Errorf("handleRemove: failed to remove webhook: %s", err)
	}
	h.ChatEcho(convID, "Success!")
	return nil
}

func (h *Handler) handleList(cmd string, msg chat1.MsgSummary) (err error) {
	convID := msg.ConvID
	hooks, err := h.db.List(convID)
	if err != nil {
		return fmt.Errorf("handleList: failed to list hook: %s", err)
	}
	err = h.checkAllowed(msg)
	switch err {
	case nil:
	case errNotAllowed:
		h.ChatEcho(convID, err.Error())
		return nil
	default:
		return err
	}

	h.stats.Count("list")
	if len(hooks) == 0 {
		h.ChatEcho(convID, "No hooks in this conversation")
		return nil
	}
	var body string
	for _, hook := range hooks {
		body += fmt.Sprintf("%s, %s\n", hook.name, h.formURL(hook.id))
	}
	if _, err := h.kbc.SendMessageByTlfName(msg.Sender.Username, body); err != nil {
		h.Debug("handleList: failed to send hook: %s", err)
	}
	h.ChatEcho(convID, "List sent to @%s", msg.Sender.Username)
	return nil
}

func validateTemplate(cmd string) (templateSrc string, err error) {
	toks := strings.Fields(cmd)

	var trigger string
	switch toks[1] {
	case "create":
		trigger = "!webhook create"
	case "update":
		trigger = "!webhook update"
	}

	// templateSrc is whatever remains after removing the trigger (i.e `!webhook create` or
	// `!webhook update`) followed by the template name, and trimming spaces. if the template
	// is empty, we'll set a default one
	name := toks[2]
	templateSrc = strings.Replace(cmd, trigger, "", 1)
	templateSrc = strings.Replace(templateSrc, name, "", 1)
	templateSrc = strings.TrimSpace(templateSrc)
	if templateSrc == "" {
		templateSrc = defaultTemplate
	}
	tWithVars := injectTemplateVars("testhook1", "POST", templateSrc)
	_, err = template.New("").Parse(tWithVars)
	if err != nil {
		return "", err
	}
	return templateSrc, nil
}

func (h *Handler) handleCreate(cmd string, msg chat1.MsgSummary) (err error) {
	convID := msg.ConvID
	toks := strings.Fields(cmd)
	if len(toks) < 3 {
		h.ChatEcho(convID, "invalid number of arguments, must specify a name")
		return nil
	}
	err = h.checkAllowed(msg)
	switch err {
	case nil:
	case errNotAllowed:
		h.ChatEcho(convID, err.Error())
		return nil
	default:
		return err
	}

	h.stats.Count("create")
	name := toks[2]
	templateSrc, err := validateTemplate(cmd)
	if err != nil {
		h.ChatEcho(convID, "failed to parse template: %v", err)
		return fmt.Errorf("handleCreate: failed to parse template: %s", err)
	}

	id, err := h.db.Create(name, templateSrc, convID)
	if err != nil {
		return fmt.Errorf("handleCreate: failed to create webhook: %s", err)
	}
	if _, err := h.kbc.SendMessageByTlfName(msg.Sender.Username, "%s", h.formURL(id)); err != nil {
		h.Debug("handleCreate: failed to send hook: %s", err)
	}
	h.ChatEcho(convID, "Success! New URL sent to @%s", msg.Sender.Username)
	return nil
}

func (h *Handler) handleUpdate(cmd string, msg chat1.MsgSummary) (err error) {
	convID := msg.ConvID
	toks := strings.Fields(cmd)
	if len(toks) < 3 {
		h.ChatEcho(convID, "invalid number of arguments, must specify a name")
		return nil
	}
	err = h.checkAllowed(msg)
	switch err {
	case nil:
	case errNotAllowed:
		h.ChatEcho(convID, err.Error())
		return nil
	default:
		return err
	}
	h.stats.Count("update")
	name := toks[2]
	templateSrc, err := validateTemplate(cmd)
	if err != nil {
		h.ChatEcho(convID, "failed to parse template: %v", err)
		return fmt.Errorf("handleUpdate: failed to parse template: %s", err)
	}

	err = h.db.Update(name, templateSrc, convID)
	switch err {
	case nil:
	case sql.ErrNoRows:
	default:
		h.ChatEcho(convID, "failed to update template: no webhook with that name exists in this conversation. a new webhook can be created with the `!webhook create` command.")
		return nil
	}
	h.ChatEcho(convID, "Success!")
	return nil
}

func (h *Handler) handleHelp(msg chat1.MsgSummary) (err error) {
	back := "`"
	backs := "```"
	convID := msg.ConvID
	h.ChatEcho(convID, `
*Templates:*
	- Information about creating templates, and some examples can be found at https://pkg.go.dev/text/template
	- You can test your templates against your JSON data at https://play.golang.org/p/vC06kRCQDfX

	There are 2 custom variables you can use in your templates (see the default template below for example usage):
	  - %s$webhookName%s: The name you gave the webhook
	  - %s$webhookMethod%s: Whether GET or POST was used when the webhook endpoint was fetched

	If you don't provide a template, this is the default:%s
	[hook: *{{$webhookName}}*]

	{{if eq $webhookMethod "POST"}}{{.msg}}{{else}}{{index .msg 0}}{{end}}%s`, back, back, back, back, backs, backs)
	return nil
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "I can create generic webhooks into Keybase! Try `!webhook create` to get started."
	return base.HandleNewTeam(h.stats, h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		return nil
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!webhook create"):
		return h.handleCreate(cmd, msg)
	case strings.HasPrefix(cmd, "!webhook list"):
		return h.handleList(cmd, msg)
	case strings.HasPrefix(cmd, "!webhook remove"):
		return h.handleRemove(cmd, msg)
	case strings.HasPrefix(cmd, "!webhook update"):
		return h.handleUpdate(cmd, msg)
	case strings.HasPrefix(cmd, "!webhook help"):
		return h.handleHelp(msg)
	}
	return nil
}
