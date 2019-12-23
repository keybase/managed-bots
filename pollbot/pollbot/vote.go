package pollbot

import (
	"encoding/hex"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type Vote struct {
	ConvID string
	MsgID  chat1.MessageID
	Choice int
}

type voteToEncode struct {
	ConvID []byte          `codec:"c"`
	MsgID  chat1.MessageID `codec:"m"`
	Choice int             `codec:"i"`
}

func NewVote(convID string, msgID chat1.MessageID, choice int) Vote {
	return Vote{
		ConvID: convID,
		MsgID:  msgID,
		Choice: choice,
	}
}

func NewVoteFromEncoded(sdat string) Vote {
	var ve voteToEncode
	dat, _ := base.URLEncoder().DecodeString(sdat)
	_ = base.MsgpackDecode(&ve, dat)
	return Vote{
		ConvID: hex.EncodeToString(ve.ConvID),
		MsgID:  ve.MsgID,
		Choice: ve.Choice,
	}
}

func (v Vote) Encode() string {
	cdat, _ := hex.DecodeString(base.ShortConvID(v.ConvID))
	mdat, _ := base.MsgpackEncode(voteToEncode{
		ConvID: cdat,
		MsgID:  v.MsgID,
		Choice: v.Choice,
	})
	return base.URLEncoder().EncodeToString(mdat)
}
