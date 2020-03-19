package zoombot

import "github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"

func IdentifierFromMsg(msg chat1.MsgSummary) string {
	return msg.Sender.Username // use username as identifier (same zoom account for every conv, per user)
}
