package macrobot

import (
	"fmt"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

const (
	back          = "`"
	backs         = "```"
	CreateCmdHelp = `You must specify a name for the macro, such as 'docs' or 'lunchflip' as well as a message for the bot to send whenever you invoke the macro.

Examples:%s
!macro create docs 'You can find documentation at: https://keybase.io/docs'
!macro create lunchflip '/flip alice, bob, charlie'%s
You can run the above macros using %s!docs%s or %s!lunchflip%s`
)

func getCreateForChannelCmd() chat1.UserBotCommandInput {
	createForChannelDesc := fmt.Sprintf("Create or update a macro for the current channel. %s",
		fmt.Sprintf(CreateCmdHelp, backs, backs, back, back, back, back))
	return chat1.UserBotCommandInput{
		Name:        "macro create-for-channel",
		Description: "Create or update a macro for the current channel",
		ExtendedDescription: &chat1.UserBotExtendedDescription{
			Title:       `*!macro create-for-channel* <name> <message>`,
			DesktopBody: createForChannelDesc,
			MobileBody:  createForChannelDesc,
		},
	}
}

func getChannelType(channel chat1.ChatChannel, isConv bool) string {
	if channel.MembersType == "team" {
		if isConv {
			return "channel"
		}
		return "team"
	}
	return "conversation"
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
