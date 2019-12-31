package base

import (
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

func ShortConvID(convID string) ShortID {
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
	if conv.Channel.MembersType == "team" && conv.Channel.TopicName != "general" {
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
