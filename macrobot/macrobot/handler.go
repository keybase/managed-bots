package macrobot

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type Handler struct {
	*base.DebugOutput

	stats *base.StatsRegistry
	kbc   *kbchat.API
	db    *DB
}

var _ base.Handler = (*Handler)(nil)

var whiteListedCommands = [...]string{
	"flip",
	"giphy",
}

func NewHandler(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig, db *DB) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		stats:       stats.SetPrefix("Handler"),
		kbc:         kbc,
		db:          db,
	}
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "I can create and run simple macros! Try `!macro create` to get started."
	return base.HandleNewTeam(h.stats, h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		return nil
	}

	cmd := strings.TrimSpace(msg.Content.Text.Body)

	if !strings.HasPrefix(cmd, "!") {
		return nil
	}

	tokens, userErr, err := base.SplitTokens(cmd)
	if err != nil {
		return err
	} else if userErr != "" {
		h.ChatEcho(msg.ConvID, userErr)
		return nil
	}

	switch {
	case strings.HasPrefix(cmd, "!macro create"):
		return h.handleCreate(msg, tokens[2:])
	case strings.HasPrefix(cmd, "!macro list"):
		return h.handleList(msg)
	case strings.HasPrefix(cmd, "!macro remove"):
		return h.handleRemove(msg, tokens[2:])
	default:
		return h.handleRun(msg, tokens)
	}
}

func (h *Handler) handleRun(msg chat1.MsgSummary, args []string) error {
	if len(args) != 1 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments. Expected one: <name>")
		return nil
	}

	macroName := strings.TrimPrefix(args[0], "!")
	macroMessage, err := h.db.Get(msg.Channel, macroName)
	switch err {
	case nil:
	case sql.ErrNoRows:
		h.ChatEcho(msg.ConvID, "Macro '%s' is not defined for this %s", macroName, getChannelType(msg.Channel))
		return nil
	default:
		return err
	}
	sanitizedMacroMessage := sanitizeMessage(macroMessage)

	h.ChatEcho(msg.ConvID, sanitizedMacroMessage)
	return nil
}

func (h *Handler) handleCreate(msg chat1.MsgSummary, args []string) error {
	if len(args) != 2 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments. Expected two: <name> <message>")
		return nil
	}

	isAllowed, err := base.IsAtLeastWriter(h.kbc, msg.Sender.Username, msg.Channel)
	if err != nil {
		return err
	}
	if !isAllowed {
		h.ChatEcho(msg.ConvID, "You must be at least a writer to configure me!")
		return nil
	}

	macroName := args[0]
	macroMessage := args[1]
	err = h.db.Create(msg.Channel, macroName, macroMessage)
	if err != nil {
		return err
	}

	err = h.doPrivateAdvertisement(msg.Channel)
	if err != nil {
		return err
	}

	h.ChatEcho(msg.ConvID, "Macro '%s' created", macroName)
	return nil
}

func (h *Handler) handleList(msg chat1.MsgSummary) error {
	macroList, err := h.db.List(msg.Channel)
	if err != nil {
		return err
	}

	if len(macroList) == 0 {
		h.ChatEcho(msg.ConvID, "There are no macros defined for this %s", getChannelType(msg.Channel))
		return nil
	}

	data := []interface{}{getChannelType(msg.Channel)}
	for _, macro := range macroList {
		data = append(data, macro.Name)
		data = append(data, macro.Message)
	}
	macroListMessage := "Here are the macros available for this %s:" + strings.Repeat("\n• %s: `%q`", len(macroList))
	h.ChatEcho(msg.ConvID, macroListMessage, data...)
	return nil
}

func (h *Handler) handleRemove(msg chat1.MsgSummary, args []string) error {
	if len(args) != 1 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments. Expected one: <name>")
		return nil
	}

	isAllowed, err := base.IsAtLeastWriter(h.kbc, msg.Sender.Username, msg.Channel)
	if err != nil {
		return err
	}
	if !isAllowed {
		h.ChatEcho(msg.ConvID, "You must be at least a writer to configure me!")
		return nil
	}

	macroName := args[0]
	err = h.db.Remove(msg.Channel, macroName)
	if err != nil {
		return err
	}

	err = h.doPrivateAdvertisement(msg.Channel)
	if err != nil {
		return err
	}

	h.ChatEcho(msg.ConvID, "Macro '%s' removed", macroName)
	return nil
}

func (h *Handler) doPrivateAdvertisement(channel chat1.ChatChannel) error {
	macroList, err := h.db.List(channel)
	if err != nil {
		return err
	}

	cmds := make([]chat1.UserBotCommandInput, len(macroList))
	for i, macro := range macroList {
		cmds[i] = chat1.UserBotCommandInput{
			Name:        macro.Name,
			Description: fmt.Sprintf("Run the '%s' macro defined for this %s", macro.Name, getChannelType(channel)),
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       fmt.Sprintf("*!%s*", macro.Name),
				DesktopBody: macro.Message,
				MobileBody:  macro.Message,
			},
		}
	}

	ad := kbchat.Advertisement{
		Advertisements: []chat1.AdvertiseCommandAPIParam{
			{
				Typ:      "teamconvs",
				Commands: cmds,
				TeamName: channel.Name,
			},
		},
	}

	_, err = h.kbc.AdvertiseCommands(ad)

	return err
}

func getChannelType(channel chat1.ChatChannel) string {
	if channel.MembersType == "team" {
		return "team"
	} else {
		return "conversation"
	}
}

func sanitizeMessage(message string) string {
	if strings.HasPrefix(message, "/") && !isWhiteListed(message) {
		// escape beginning slash
		message = "\\" + message
	}
	// prevent stellar payments by escaping all '+'s
	message = strings.ReplaceAll(message, "+", "\\+")
	return message
}

func isWhiteListed(message string) bool {
	for _, command := range whiteListedCommands {
		if strings.HasPrefix(message, "/"+command) {
			return true
		}
	}
	return false
}
