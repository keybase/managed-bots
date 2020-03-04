package base

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/kballard/go-shellquote"

	"github.com/keybase/go-codec/codec"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

const backs = "```"

type ShortID string

var DefaultBotAdmins = []string{
	"01",
	"joshblum",
	"marceloneil",
	"mikem",
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

func HandleNewTeam(stats *StatsRegistry, log *DebugOutput, kbc *kbchat.API, conv chat1.ConvSummary, welcomeMsg string) error {
	if conv.Channel.MembersType == "team" && !conv.IsDefaultConv {
		log.Debug("HandleNewTeam: skipping conversation %+v, not default team conv", conv)
		stats.Count("HandleNewTeam - skipped new conv")
		return nil
	} else if conv.CreatorInfo != nil && conv.CreatorInfo.Username == kbc.GetUsername() {
		log.Debug("HandleNewTeam: skipping conversation %+v, bot created conversation", conv)
		stats.Count("HandleNewTeam - skipped new conv")
		return nil
	}
	stats.Count("HandleNewTeam - new conv")
	_, err := kbc.SendMessageByConvID(conv.Id, welcomeMsg)
	return err
}

func IsAtLeastWriter(kbc *kbchat.API, senderUsername string, channel chat1.ChatChannel) (bool, error) {
	switch channel.MembersType {
	case "team": // make sure the member is an admin or owner
	default: // authorization is per user so let anything through
		return true, nil
	}
	res, err := kbc.ListMembersOfTeam(channel.Name)
	if err != nil {
		return false, err
	}
	allowed := append(append(res.Owners, res.Admins...), res.Writers...)
	for _, member := range allowed {
		if member.Username == senderUsername {
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
	min-height: 94vh;
	flex-direction: column;
	padding-top: 50px;
	font-family: 'Lucida Sans', 'Lucida Sans Regular', 'Lucida Grande', 'Lucida Sans Unicode', Geneva,
		Verdana, sans-serif;
	font-size: 22px;
}
main{
	flex: 1;
}
.content{
	display: flex;
	flex-direction: column;
	justify-content: center;
	align-items: center;
}
a {
	color: black;
}
.msg {
	text-align: center;
	padding-top: 15px;
	padding-bottom: 7px;
}
.logo {
	width: 150px;
	padding: 25px;
}
.success {
	font-size: 32px;
	margin-bottom: 24px;
}
footer {
	font-size: 15px;
}
@media only screen and (max-width: 768px) {
	body {
		font-size: 18px;
	}
	footer {
		font-size: 12px;
	}
	.logo {
		width: 100px;
		padding: 20px;
	}
}
</style>
<title>` + botName + ` | ` + title + `</title>
</head>
<body>
  <main>
	<div class="content">
	  <a href="https://keybase.io"><img class="logo" src="` + logoUrl + `"></a>
	  <div>
		<div class="msg">` + msg + `</div>
	  </div>
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

func RandHexString(length int) string {
	b, err := RandBytes(length)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(b)
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

// Secret token given to API's for authentication (after establishing webhooks)
// We expect them to return this secret token in webhook POST requests for validation
func MakeSecret(repo string, convID chat1.ConvIDStr, secret string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(repo+string(ShortConvID(convID))+secret)))
}

func SendByConvNameOrID(kbc *kbchat.API, debugOutput *DebugOutput, name, msg string, args ...interface{}) (err error) {
	if _, err = kbc.SendMessageByConvID(chat1.ConvIDStr(name), msg, args...); err == nil {
		return nil
	}
	debugOutput.Debug("unable to send by ConvID: %v", err)
	if _, err = kbc.SendMessageByTlfName(name, msg, args...); err == nil {
		return nil
	}
	debugOutput.Debug("unable to send by tlfName: %v", err)
	if _, err = kbc.SendMessageByTeamName(name, nil, msg, args...); err == nil {
		return nil
	}
	debugOutput.Debug("unable to send by teamName: %v", err)
	return err
}

func SplitTokens(cmd string) (tokens []string, userErrorMessage string, err error) {
	tokens, err = shellquote.Split(cmd)
	switch err {
	case nil:
		return tokens, "", nil
	case shellquote.UnterminatedSingleQuoteError,
		shellquote.UnterminatedDoubleQuoteError,
		shellquote.UnterminatedEscapeError:
		return nil, fmt.Sprintf("Error in command: %s", strings.ToLower(err.Error())), nil
	default:
		return nil, "", fmt.Errorf("error splitting command string: %s", err)
	}
}

func IsDirectPrivateMessage(botUsername, senderUsername string, channel chat1.ChatChannel) bool {
	if channel.MembersType == "team" {
		return false
	}
	if senderUsername == channel.Name {
		return true
	}
	if len(strings.Split(channel.Name, ",")) == 2 {
		if strings.Contains(channel.Name, botUsername+",") ||
			strings.Contains(channel.Name, ","+botUsername) {
			return true
		}
	}
	return false
}

func feedbackCmd(prefix string) string {
	return fmt.Sprintf("%s feedback", prefix)
}

func GetFeedbackCommandAdvertisement(prefix string) chat1.UserBotCommandInput {
	feedbackExtended := fmt.Sprintf(`Let us know if you run into an issue or would like to see a new feature.

Examples:%s
!%s I got this error but I'm not sure what I did wrong...
!%s Looking great!
%s
	`, backs, feedbackCmd(prefix), feedbackCmd(prefix), backs)
	return chat1.UserBotCommandInput{
		Name:        feedbackCmd(prefix),
		Description: "Tell us how we're doing!",
		ExtendedDescription: &chat1.UserBotExtendedDescription{
			Title:       fmt.Sprintf("*!%s*", feedbackCmd(prefix)),
			DesktopBody: feedbackExtended,
			MobileBody:  feedbackExtended,
		},
	}
}
