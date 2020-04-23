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
	case strings.HasPrefix(cmd, "!macro create "):
		return h.handleCreate(msg, false, tokens[2:])
	case strings.HasPrefix(cmd, "!macro create-for-channel"):
		return h.handleCreate(msg, true, tokens[2:])
	case strings.HasPrefix(cmd, "!macro list"):
		return h.handleList(msg)
	case strings.HasPrefix(cmd, "!macro remove"):
		return h.handleRemove(msg, tokens[2:])
	default:
		return h.handleRun(msg, tokens)
	}
}

func (h *Handler) handleRun(msg chat1.MsgSummary, args []string) error {
	macroName := strings.TrimPrefix(args[0], "!")
	macroMessage, err := h.db.Get(msg, macroName)
	switch err {
	case nil:
	case sql.ErrNoRows:
		return nil
	default:
		return err
	}
	sanitizedMacroMessage := sanitizeMessage(macroMessage)
	h.ChatEcho(msg.ConvID, sanitizedMacroMessage)
	return nil
}

func (h *Handler) handleCreate(msg chat1.MsgSummary, isConv bool, args []string) error {
	if len(args) != 2 {
		h.ChatEcho(msg.ConvID, "Invalid number of arguments. Expected two: <name> <message>")
		return nil
	}

	isAllowed, err := base.IsAtLeastWriter(h.kbc, msg.Sender.Username, msg.Channel)
	if err != nil {
		return err
	} else if !isAllowed {
		h.ChatEcho(msg.ConvID, "You must be at least a writer to configure me!")
		return nil
	}

	macroName := args[0]
	macroMessage := args[1]
	created, err := h.db.Create(msg, isConv, macroName, macroMessage)
	if err != nil {
		return err
	}

	if err = h.doPrivateAdvertisement(msg); err != nil {
		return err
	}
	if created {
		h.ChatEcho(msg.ConvID, "Created '%s'.", macroName)
	} else {
		h.ChatEcho(msg.ConvID, "Updated '%s'.", macroName)
	}
	return nil
}

func (h *Handler) handleList(msg chat1.MsgSummary) error {
	macroList, err := h.db.List(msg)
	if err != nil {
		return err
	} else if len(macroList) == 0 {
		h.ChatEcho(msg.ConvID, "There are no macros defined for this %s", getChannelType(msg.Channel, true))
		return nil
	}

	data := []interface{}{getChannelType(msg.Channel, true)}
	hasConvs := false
	for i, macro := range macroList {
		// If we have a team and conv command defined with the same name we
		// only show the conv version. The DB orders convs first for us.
		if i > 0 && macroList[i-1].Name == macro.Name {
			continue
		}
		if macro.IsConv {
			data = append(data, fmt.Sprintf("\\*\\*%s", macro.Name))
			hasConvs = true
		} else {
			data = append(data, macro.Name)
		}
		data = append(data, macro.Message)
	}
	macroListMessage := "Here are the macros available for this %s:" + strings.Repeat("\n• %s: `%q`", len(data)/2)
	if hasConvs {
		macroListMessage += "\n\t\\*\\*restricted to this %s"
		data = append(data, getChannelType(msg.Channel, true))
	}
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
	} else if !isAllowed {
		h.ChatEcho(msg.ConvID, "You must be at least a writer to configure me!")
		return nil
	}

	macroName := args[0]
	removed, err := h.db.Remove(msg, macroName)
	if err != nil {
		return err
	}

	if err = h.doPrivateAdvertisement(msg); err != nil {
		return err
	}

	if removed {
		h.ChatEcho(msg.ConvID, "Removed '%s'.", macroName)
	} else {
		h.ChatEcho(msg.ConvID, "'%s' does not exist.", macroName)
	}
	return nil
}

func (h *Handler) doPrivateAdvertisement(msg chat1.MsgSummary) error {
	macroList, err := h.db.List(msg)
	if err != nil {
		return err
	}

	var teamCmds, convCmds []chat1.UserBotCommandInput
	for _, macro := range macroList {
		cmd := chat1.UserBotCommandInput{
			Name:        macro.Name,
			Description: fmt.Sprintf("Run the '%s' macro defined for this %s", macro.Name, getChannelType(msg.Channel, macro.IsConv)),
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       fmt.Sprintf("*!%s*", macro.Name),
				DesktopBody: macro.Message,
				MobileBody:  macro.Message,
			},
		}
		if macro.IsConv {
			convCmds = append(convCmds, cmd)
		} else {
			teamCmds = append(teamCmds, cmd)
		}
	}
	var ad kbchat.Advertisement
	if len(teamCmds) > 0 {
		ad.Advertisements = append(ad.Advertisements, chat1.AdvertiseCommandAPIParam{
			Typ:      "teamconvs",
			Commands: teamCmds,
			TeamName: msg.Channel.Name,
		})
	}
	if len(convCmds) > 0 {
		ad.Advertisements = append(ad.Advertisements, chat1.AdvertiseCommandAPIParam{
			Typ:      "conv",
			Commands: convCmds,
			ConvID:   msg.ConvID,
		})
	}

	_, err = h.kbc.AdvertiseCommands(ad)
	return err
}

func getChannelType(channel chat1.ChatChannel, isConv bool) string {
	if channel.MembersType == "team" {
		if isConv {
			return "channel"
		}
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
