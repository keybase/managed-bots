package meetbot

import (
	"math/rand"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

// identifierFromMsg returns either the team's name or sender's username, which
// is used to identify the oauth token. This is so we can have a separate oauth
// token per team (perhaps with a workplace account) and use a personal account
// for other events.
func identifierFromMsg(msg chat1.MsgSummary) string {
	switch msg.Channel.MembersType {
	case "team":
		return msg.Channel.Name
	default:
		return msg.Sender.Username
	}
}

func randomID(n int) string {
	letter := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

func requestID() string {
	return randomID(10)
}

func asHTML(title, msg string) []byte {
	return []byte(`
<html>
<title> meetbot | ` + title + `</title>
<body style="background-color: rgb(80,160,247);">
  <div style="margin-top: 10%;">
    <h2 style="text-align:center;color: white;">` + msg + `</h2>
  </div>
</body>
</html>
`)
}
