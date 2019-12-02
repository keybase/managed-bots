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
)

type Handler struct {
	kbc        *kbchat.API
	db         *DB
	httpSrv    *HTTPSrv
	httpPrefix string
}

func NewHandler(kbc *kbchat.API, httpSrv *HTTPSrv, db *DB, httpPrefix string) *Handler {
	return &Handler{
		kbc:        kbc,
		db:         db,
		httpSrv:    httpSrv,
		httpPrefix: httpPrefix,
	}
}

func (h *Handler) debug(msg string, args ...interface{}) {
	fmt.Printf("Handler: "+msg+"\n", args...)
}

func (h *Handler) chatDebug(convID, msg string, args ...interface{}) {
	h.debug(msg, args...)
	if _, err := h.kbc.SendMessageByConvID(convID, msg, args...); err != nil {
		h.debug("chatDebug: failed to send error message: %s", err)
	}
}

func (h *Handler) Listen() {
	sub, err := h.kbc.ListenForNewTextMessages()
	if err != nil {
		h.debug("Listen: failed to listen: %s", err)
	}
	h.debug("startup success, listening for messages...")
	for {
		msg, err := sub.Read()
		if err != nil {
			h.debug("Listen: Read() error: %s", err)
			continue
		}
		h.handleCommand(msg.Message)
	}
}

func (h *Handler) generateVoteLink(convID string, msgID chat1.MessageID, choice int) string {
	vote := NewVote(convID, msgID, choice)
	link := h.httpPrefix + "/pollbot/vote?=" + url.QueryEscape(vote.Encode())
	return strings.ReplaceAll(link, "%", "%%")
}

func (h *Handler) generateAnonymousPoll(convID string, msgID chat1.MessageID, prompt string,
	options []string) {
	body := fmt.Sprintf("Anonymous Poll: *%s*\n\n", prompt)
	for index, option := range options {
		body += fmt.Sprintf("%s  %s\n", numberToEmoji(index+1), option)
	}
	body += "\nVisit one of the following links below to vote for your choice.\n"
	sendRes, err := h.kbc.SendMessageByConvID(convID, body)
	if err != nil {
		h.chatDebug(convID, "failed to send poll: %s", err)
		return
	}
	if sendRes.Result.MessageID == nil {
		h.chatDebug(convID, "failed to get ID of prompt message")
		return
	}
	promptMsgID := *sendRes.Result.MessageID
	var choiceBody string
	for index := range options {
		choiceBody += numberToEmoji(index+1) + "  " + h.generateVoteLink(convID, promptMsgID, index+1) + "\n"
	}
	if sendRes, err = h.kbc.SendMessageByConvID(convID, choiceBody); err != nil {
		h.chatDebug(convID, "failed to send poll: %s", err)
		return
	}
	if sendRes.Result.MessageID == nil {
		h.chatDebug(convID, "failed to get ID of choice message")
		return
	}
	if sendRes, err = h.kbc.SendMessageByConvID(convID, "*Results*\n_No votes yet_"); err != nil {
		h.chatDebug(convID, "failed to send poll: %s", err)
		return
	}
	if sendRes.Result.MessageID == nil {
		h.chatDebug(convID, "failed to get ID of result message")
		return
	}
	resultMsgID := *sendRes.Result.MessageID
	if err := h.db.CreatePoll(convID, promptMsgID, resultMsgID); err != nil {
		h.chatDebug(convID, "failed to create poll: %s", err)
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
		h.chatDebug(convID, "failed to send poll: %s", err)
		return
	}
	if sendRes.Result.MessageID == nil {
		h.chatDebug(convID, "failed to get ID of prompt message")
		return
	}
	for index := range options {
		if _, err := h.kbc.ReactByConvID(convID, *sendRes.Result.MessageID,
			numberToEmoji(index+1)); err != nil {
			h.chatDebug(convID, "failed to set reaction option: %s", err)
		}
	}
}

func (h *Handler) handlePoll(cmd, convID string, msgID chat1.MessageID) {
	toks, err := shellquote.Split(cmd)
	if err != nil {
		h.chatDebug(convID, "failed to parse poll command: %s", err)
		return
	}
	var anonymous bool
	flags := flag.NewFlagSet(toks[0], flag.ContinueOnError)
	flags.BoolVar(&anonymous, "anonymous", false, "")
	if err := flags.Parse(toks[1:]); err != nil {
		h.chatDebug(convID, "failed to parse poll command: %s", err)
		return
	}
	args := flags.Args()
	if len(args) < 2 {
		h.chatDebug(convID, "must specify a prompt and at least one option")
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
		h.debug("failed to send login attempt: %s", err)
		return
	}
}

func (h *Handler) handleCommand(msg chat1.MsgSummary) {
	if msg.Content.Text == nil {
		h.debug("skipping non-text message")
		return
	}
	cmd := strings.Trim(msg.Content.Text.Body, " ")
	switch {
	case strings.HasPrefix(cmd, "!poll"):
		h.handlePoll(cmd, msg.ConvID, msg.Id)
	case cmd == "login":
		h.handleLogin(msg.Channel.Name, msg.Sender.Username)
	default:
		h.debug("ignoring unknown command")
	}
}
