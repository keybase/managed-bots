package pollbot

import (
	"flag"
	"fmt"
	"net/url"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/kballard/go-shellquote"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type Handler struct {
	*base.Handler
	kbc        *kbchat.API
	db         *DB
	httpSrv    *HTTPSrv
	httpPrefix string
}

func NewHandler(kbc *kbchat.API, httpSrv *HTTPSrv, db *DB, httpPrefix string) *Handler {
	h := &Handler{
		kbc:        kbc,
		db:         db,
		httpSrv:    httpSrv,
		httpPrefix: httpPrefix,
	}
	h.Handler = base.NewHandler(kbc, h)
	return h
}

func (h *Handler) generateVoteLink(convID string, msgID chat1.MessageID, choice int) string {
	vote := NewVote(convID, msgID, choice)
	link := h.httpPrefix + "/pollbot/vote?=" + url.QueryEscape(vote.Encode())
	return strings.ReplaceAll(link, "%", "%%")
}

func (h *Handler) generateAnonymousPoll(convID string, msgID chat1.MessageID, prompt string,
	options []string) {
	promptBody := fmt.Sprintf("Anonymous Poll: *%s*\n\n", prompt)
	sendRes, err := h.kbc.SendMessageByConvID(convID, promptBody)
	if err != nil {
		h.ChatDebug(convID, "failed to send poll: %s", err)
		return
	}
	if sendRes.Result.MessageID == nil {
		h.ChatDebug(convID, "failed to get ID of prompt message")
		return
	}
	promptMsgID := *sendRes.Result.MessageID
	var body string
	for index, option := range options {
		body += fmt.Sprintf("\n%s  *%s*\n%s\n", numberToEmoji(index+1), option,
			h.generateVoteLink(convID, promptMsgID, index+1))
	}
	if _, err = h.kbc.SendMessageByConvID(convID, body); err != nil {
		h.ChatDebug(convID, "failed to send choices: %s", err)
		return
	}
	if sendRes, err = h.kbc.SendMessageByConvID(convID, "*Results*\n_No votes yet_"); err != nil {
		h.ChatDebug(convID, "failed to send poll: %s", err)
		return
	}
	if sendRes.Result.MessageID == nil {
		h.ChatDebug(convID, "failed to get ID of result message")
		return
	}
	resultMsgID := *sendRes.Result.MessageID
	if err := h.db.CreatePoll(convID, promptMsgID, resultMsgID, len(options)); err != nil {
		h.ChatDebug(convID, "failed to create poll: %s", err)
		return
	}
}

func (h *Handler) generatePoll(convID string, msgID chat1.MessageID, prompt string,
	options []string) {
	body := fmt.Sprintf("Poll: *%s*\n\n", prompt)
	for index, option := range options {
		body += fmt.Sprintf("%s  %s\n", numberToEmoji(index+1), option)
	}
	body += "Tap a reaction below to register your vote!"
	sendRes, err := h.kbc.SendMessageByConvID(convID, body)
	if err != nil {
		h.ChatDebug(convID, "failed to send poll: %s", err)
		return
	}
	if sendRes.Result.MessageID == nil {
		h.ChatDebug(convID, "failed to get ID of prompt message")
		return
	}
	for index := range options {
		if _, err := h.kbc.ReactByConvID(convID, *sendRes.Result.MessageID,
			numberToEmoji(index+1)); err != nil {
			h.ChatDebug(convID, "failed to set reaction option: %s", err)
		}
	}
}

func (h *Handler) handlePoll(cmd, convID string, msgID chat1.MessageID) {
	toks, err := shellquote.Split(cmd)
	if err != nil {
		h.ChatDebug(convID, "failed to parse poll command: %s", err)
		return
	}
	var anonymous bool
	flags := flag.NewFlagSet(toks[0], flag.ContinueOnError)
	flags.BoolVar(&anonymous, "anonymous", false, "")
	if err := flags.Parse(toks[1:]); err != nil {
		h.ChatDebug(convID, "failed to parse poll command: %s", err)
		return
	}
	args := flags.Args()
	if len(args) < 2 {
		h.ChatDebug(convID, "must specify a prompt and at least one option")
	}
	prompt := args[0]
	if anonymous {
		h.generateAnonymousPoll(convID, msgID, prompt, args[1:])
	} else {
		h.generatePoll(convID, msgID, prompt, args[1:])
	}
}

func (h *Handler) handleLogin(convName, username string) {
	// make sure we are in a conv with just the person
	if !(convName == fmt.Sprintf("%s,%s", username, h.kbc.GetUsername()) ||
		convName == fmt.Sprintf("%s,%s", h.kbc.GetUsername(), username)) {
		return
	}
	token := h.httpSrv.LoginToken(username)
	body := fmt.Sprintf(`Thanks for using the Keybase polling service!

To login your web browser in order to vote in anonymous polls, please follow the link below. Once that is completed, you will be able to vote in anonymous polls simply by clicking the links that I provide in the polls.

%s`, fmt.Sprintf("%s/pollbot/login?token=%s&username=%s", h.httpPrefix, token, username))
	if _, err := h.kbc.SendMessageByTlfName(username, body); err != nil {
		h.Debug("failed to send login attempt: %s", err)
		return
	}
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		h.Debug("skipping non-text message")
		return nil
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!poll"):
		h.handlePoll(cmd, msg.ConvID, msg.Id)
	case cmd == "login":
		h.handleLogin(msg.Channel.Name, msg.Sender.Username)
	default:
		h.Debug("ignoring unknown command: %q", cmd)
	}
	return nil
}
