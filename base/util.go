package base

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
		log.Debug("HandleNewTeam: skipping conversation %+v, not default team conv", conv)
		return nil
	} else if conv.CreatorInfo != nil && conv.CreatorInfo.Username == kbc.GetUsername() {
		log.Debug("HandleNewTeam: skipping conversation %+v, bot created conversation", conv)
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

func IsDirectPrivateMessage(ownUsername string, msg chat1.MsgSummary) bool {
	if msg.Channel.MembersType == "team" {
		return false
	}
	if msg.Sender.Username == msg.Channel.Name {
		return true
	}
	if len(strings.Split(msg.Channel.Name, ",")) == 2 {
		if strings.Contains(msg.Channel.Name, ownUsername+",") ||
			strings.Contains(msg.Channel.Name, ","+ownUsername) {
			return true
		}
	}
	return false
}

func GetFeedbackCommandAdvertisement(prefix string) chat1.UserBotCommandInput {
	feedbackExtended := fmt.Sprintf(`Let us know if you run into an issue or would like to see a new feature.

Examples:%s
!%s feedback I got this error but I'm not sure what I did wrong...
!%s feedback Looking great!
%s
	`, backs, prefix, prefix, backs)
	return chat1.UserBotCommandInput{
		Name:        fmt.Sprintf("%s feedback", prefix),
		Description: "Tell us how we're doing!",
		ExtendedDescription: &chat1.UserBotExtendedDescription{
			Title:       fmt.Sprintf("*!%s feedback*", prefix),
			DesktopBody: feedbackExtended,
			MobileBody:  feedbackExtended,
		},
	}
}
