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
<head>
<style>
body {
	background-color: rgb(80,160,247);
	display: flex;
	min-height: 99vh;
	flex-direction: column;
}
.content{
	flex: 1;
}
.msg {
	text-align: center;
	color: white;
	margin-top: 20vh;
}

a {
	color: white;
}
</style>
<title> meetbot | ` + title + `</title>
</head>
<body>
  <main class="content">
	  <div>
		<h1 class="msg">` + msg + `</h1>
	  </div>
  </main>
  <footer>
		<a href="https://keybase.io/docs/privacypolicy">Privacy Policy</a>
  </footer>
</body>
</html>
`)
}
