package pollbot

import (
	"flag"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/kballard/go-shellquote"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type Handler struct {
	kbc        *kbchat.API
	db         *DB
	httpPrefix string
}

func NewHandler(kbc *kbchat.API, db *DB, httpPrefix string) *Handler {
	return &Handler{
		kbc:        kbc,
		db:         db,
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
			h.debug("Listen: Read() error: %s", err.Error())
			continue
		}
		h.handleCommand(msg.Message)
	}
}

func (h *Handler) numberToEmoji(v int) string {
	switch v {
	case 1:
		return ":one:"
	case 2:
		return ":two:"
	case 3:
		return ":three:"
	case 4:
		return ":four:"
	case 5:
		return ":five:"
	case 6:
		return ":six:"
	case 7:
		return ":seven:"
	case 8:
		return ":eight:"
	case 9:
		return ":nine:"
	case 10:
		return ":ten:"
	default:
		return fmt.Sprintf("%d", v)
	}
}

func (h *Handler) generateVoteLink(convID string, msgID chat1.MessageID, choice int) string {
	vote := NewVote(convID, msgID, choice)
	return fmt.Sprintf("%s/pollbot/vote?=%s", h.httpPrefix, vote.Encode())
}

func (h *Handler) generateAnonymousPoll(convID string, msgID chat1.MessageID, prompt string,
	options []string) {
	body := fmt.Sprintf("Anonymous Poll: *%s*\n\n", prompt)
	for index, option := range options {
		body += fmt.Sprintf("%s  %s\n", h.numberToEmoji(index+1), option)
	}
	body += "\nHit one of the links below to vote for an option"
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
		choiceBody += fmt.Sprintf("%s  %s\n", h.numberToEmoji(index+1),
			h.generateVoteLink(convID, promptMsgID, index+1))
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
		body += fmt.Sprintf("%s  %s\n", h.numberToEmoji(index+1), option)
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
			h.numberToEmoji(index+1)); err != nil {
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

func (h *Handler) formatTally(tally Tally) (res string) {
	res = "*Results*\n"
	if len(tally) == 0 {
		res += "_No votes yet_"
		return res
	}
	for _, t := range tally {
		s := ""
		if t.votes > 1 {
			s = "s"
		}
		res += fmt.Sprintf("%s  `%d vote%s`\n", h.numberToEmoji(t.choice), t.votes, s)
	}
	return res
}

func (h *Handler) handleReactConfirm(convID, username string, reaction chat1.MessageReaction) {
	if reaction.Body != ":white_check_mark:" {
		return
	}
	vote, err := h.db.GetStagedVote(username, reaction.MessageID)
	if err != nil {
		h.chatDebug(convID, "failed to find user vote: %s", err)
		return
	}
	if err := h.db.CastVote(username, vote, reaction.MessageID); err != nil {
		h.chatDebug(convID, "failed to cast vote: %s", err)
		return
	}
	resultMsgID, err := h.db.GetPollResultMsgID(vote.ConvID, vote.MsgID)
	if err != nil {
		h.chatDebug(convID, "failed to find poll result msg: %s", err)
		return
	}
	tally, err := h.db.GetTally(vote.ConvID, vote.MsgID)
	if err != nil {
		h.chatDebug(convID, "failed to get tally: %s", err)
		return
	}
	if _, err := h.kbc.EditByConvID(vote.ConvID, resultMsgID, h.formatTally(tally)); err != nil {
		h.chatDebug(convID, "failed to post result: %s", err)
		return
	}
	if _, err := h.kbc.SendMessageByConvID(convID, "Congratulations! Vote recorded successfully."); err != nil {
		h.debug("failed to send congrats: %s", err)
	}
}

func (h *Handler) handleCommand(msg chat1.MsgSummary) {
	if msg.Content.Reaction != nil && msg.Sender.Username != h.kbc.GetUsername() {
		h.debug("handling reaction")
		h.handleReactConfirm(msg.ConvID, msg.Sender.Username, *msg.Content.Reaction)
		return
	}

	if msg.Content.Text == nil {
		h.debug("skipping non-text message")
		return
	}
	cmd := strings.Trim(msg.Content.Text.Body, " ")
	switch {
	case strings.HasPrefix(cmd, "!poll"):
		h.handlePoll(cmd, msg.ConvID, msg.Id)
	default:
		h.debug("ignoring unknown command")
	}
}
