package base

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/keybase/go-codec/codec"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type ShortID string

var DefaultBotAdmins = []string{
	"joshblum",
	"mikem",
	"01",
}

func MsgpackDecode(dst interface{}, src []byte) error {
	h := codecHandle()
	return codec.NewDecoderBytes(src, h).Decode(dst)
}

func MsgpackEncode(src interface{}) ([]byte, error) {
	h := codecHandle()
	var ret []byte
	err := codec.NewEncoderBytes(&ret, h).Encode(src)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func codecHandle() *codec.MsgpackHandle {
	var mh codec.MsgpackHandle
	mh.WriteExt = true
	return &mh
}

func ShortConvID(convID chat1.ConvIDStr) ShortID {
	if len(convID) <= 20 {
		return ShortID(convID)
	}
	return ShortID(convID[:20])
}

func URLEncoder() *base64.Encoding {
	return base64.URLEncoding.WithPadding(base64.NoPadding)
}

func NumberToEmoji(v int) string {
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

func EmojiToNumber(s string) int {
	switch s {
	case ":one:":
		return 1
	case ":two:":
		return 2
	case ":three:":
		return 3
	case ":four:":
		return 4
	case ":five:":
		return 5
	case ":six:":
		return 6
	case ":seven:":
		return 7
	case ":eight:":
		return 8
	case ":nine:":
		return 9
	case ":ten:":
		return 10
	default:
		return 0
	}
}

func HandleNewTeam(log *DebugOutput, kbc *kbchat.API, conv chat1.ConvSummary, welcomeMsg string) error {
	if conv.Channel.MembersType == "team" && !conv.IsDefaultConv {
		log.Debug("HandleNewTeam: skipping conversation %+v", conv)
		return nil
	}
	_, err := kbc.SendMessageByConvID(conv.Id, welcomeMsg)
	return err
}

func IsAdmin(kbc *kbchat.API, msg chat1.MsgSummary) (bool, error) {
	switch msg.Channel.MembersType {
	case "team": // make sure the member is an admin or owner
	default: // authorization is per user so let anything through
		return true, nil
	}
	res, err := kbc.ListMembersOfTeam(msg.Channel.Name)
	if err != nil {
		return false, err
	}
	adminLike := append(res.Owners, res.Admins...)
	for _, member := range adminLike {
		if member.Username == msg.Sender.Username {
			return true, nil
		}
	}
	return false, nil
}

func MakeOAuthHTML(botName string, title, msg string, logoUrl string) []byte {
	return []byte(`
<html>
<head>
<style>
body {
	background-color: white;
	display: flex;
	min-height: 98vh;
	flex-direction: column;
}
.content{
	flex: 1;
}
.msg {
	text-align: center;
	color: rgb(80,160,247);
	margin-top: 15vh;
}
a {
	color: rgb(80,160,247);
}
.logo {
	width: 80px;
	padding: 5px;
}
</style>
<title>` + botName + ` | ` + title + `</title>
</head>
<body>
  <main class="content">
	  <a href="https://keybase.io"><img class="logo" src="` + logoUrl + `"></a>
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

func RandBytes(length int) ([]byte, error) {
	var n int
	var err error
	buf := make([]byte, length)
	if n, err = rand.Read(buf); err != nil {
		return nil, err
	}
	// rand.Read uses io.ReadFull internally, so this check should never fail.
	if n != length {
		return nil, fmt.Errorf("RandBytes got too few bytes, %d < %d", n, length)
	}
	return buf, nil
}

func MakeRequestID() (string, error) {
	bytes, err := RandBytes(16)
	if err != nil {
		return "", err
	}
	return URLEncoder().EncodeToString(bytes), nil
}

// identifierFromMsg returns either the team's name or sender's username, which
// is used to identify the oauth token. This is so we can have a separate oauth
// token per team (perhaps with a workplace account) and use a personal account
// for other events.
func IdentifierFromMsg(msg chat1.MsgSummary) string {
	switch msg.Channel.MembersType {
	case "team":
		return msg.Channel.Name
	default:
		return msg.Sender.Username
	}
}
