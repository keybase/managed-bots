package pollbot

import (
	"flag"
	"fmt"
	"net/url"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

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

func (h *Handler) generateVoteLink(id string, choice int) string {
	vote := NewVote(id, choice)
	link := h.httpPrefix + "/pollbot/vote?=" + url.QueryEscape(vote.Encode())
	return strings.ReplaceAll(link, "%", "%%")
}

func (h *Handler) generateAnonymousPoll(convID chat1.ConvIDStr, msgID chat1.MessageID, prompt string,
	options []string) error {
	id := base.RandHexString(8)
	promptBody := fmt.Sprintf("Anonymous Poll: *%s*\n\n", prompt)
	sendRes, err := h.kbc.SendMessageByConvID(convID, promptBody)
	if err != nil {
		return fmt.Errorf("failed to send poll: %s", err)
	}
	if sendRes.Result.MessageID == nil {
		return fmt.Errorf("failed to get ID of prompt message")
	}
	promptMsgID := *sendRes.Result.MessageID
	var body string
	for index, option := range options {
		body += fmt.Sprintf("\n%s  *%s*\n%s\n", base.NumberToEmoji(index+1), option,
			h.generateVoteLink(id, index+1))
	}
	h.ChatEcho(convID, body)
	if sendRes, err = h.kbc.SendMessageByConvID(convID, "*Results*\n_No votes yet_"); err != nil {
		return fmt.Errorf("failed to send poll: %s", err)
	}
	if sendRes.Result.MessageID == nil {
		return fmt.Errorf("failed to get ID of result message")
	}
	resultMsgID := *sendRes.Result.MessageID
	if err := h.db.CreatePoll(id, convID, promptMsgID, resultMsgID, len(options)); err != nil {
		return fmt.Errorf("failed to create poll: %s", err)
	}
	return nil
}

func (h *Handler) generatePoll(convID chat1.ConvIDStr, msgID chat1.MessageID, prompt string,
	options []string) error {
	body := fmt.Sprintf("Poll: *%s*\n\n", prompt)
	for index, option := range options {
		body += fmt.Sprintf("%s  %s\n", base.NumberToEmoji(index+1), option)
	}
	body += "Tap a reaction below to register your vote!"
	sendRes, err := h.kbc.SendMessageByConvID(convID, body)
	if err != nil {
		return fmt.Errorf("failed to send poll: %s", err)
	}
	if sendRes.Result.MessageID == nil {
		return fmt.Errorf("failed to get ID of prompt message")
	}
	for index := range options {
		if _, err := h.kbc.ReactByConvID(convID, *sendRes.Result.MessageID,
			base.NumberToEmoji(index+1)); err != nil {
			h.ChatErrorf(convID, "failed to set reaction option: %s", err)
		}
	}
	return nil
}

func (h *Handler) handlePoll(cmd string, convID chat1.ConvIDStr, msgID chat1.MessageID) error {
	toks, userErr, err := base.SplitTokens(cmd)
	if err != nil {
		return err
	} else if userErr != "" {
		h.ChatEcho(convID, userErr)
		return nil
	}
	var anonymous bool
	flags := flag.NewFlagSet(toks[0], flag.ContinueOnError)
	flags.BoolVar(&anonymous, "anonymous", false, "")
	if err := flags.Parse(toks[1:]); err != nil {
		h.ChatEcho(convID, "failed to parse poll command: %s", err)
		return nil
	}
	args := flags.Args()
	if len(args) < 2 {
		h.ChatEcho(convID, "must specify a prompt and at least one option")
		return nil
	}
	prompt := args[0]
	h.stats.Count("handlePoll")
	if anonymous {
		h.stats.Count("handlePoll - anonymous")
		return h.generateAnonymousPoll(convID, msgID, prompt, args[1:])
	} else {
		return h.generatePoll(convID, msgID, prompt, args[1:])
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

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "Find out the answers to the hardest questions. Try `!poll 'Should we move the office to a beach?' Yes No`"
	return base.HandleNewTeam(h.stats, h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		return nil
	}
	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!poll"):
		return h.handlePoll(cmd, msg.ConvID, msg.Id)
	case strings.ToLower(cmd) == "login":
		h.handleLogin(msg.Channel.Name, msg.Sender.Username)
	}
	return nil
}
