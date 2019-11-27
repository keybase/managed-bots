package pollbot

import (
	"encoding/base64"
	"encoding/hex"

	"github.com/keybase/go-codec/codec"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
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
	dat, _ := base64.StdEncoding.DecodeString(sdat)
	msgpackDecode(&ve, dat)
	return Vote{
		ConvID: hex.EncodeToString(ve.ConvID),
		MsgID:  ve.MsgID,
		Choice: ve.Choice,
	}
}

func (v Vote) Encode() string {
	cdat, _ := hex.DecodeString(v.ConvID)
	mdat, _ := msgpackEncode(voteToEncode{
		ConvID: cdat,
		MsgID:  v.MsgID,
		Choice: v.Choice,
	})
	return base64.StdEncoding.EncodeToString(mdat)
}

func codecHandle() *codec.MsgpackHandle {
	var mh codec.MsgpackHandle
	mh.WriteExt = true
	return &mh
}

func msgpackDecode(dst interface{}, src []byte) error {
	h := codecHandle()
	return codec.NewDecoderBytes(src, h).Decode(dst)
}

func msgpackEncode(src interface{}) ([]byte, error) {
	h := codecHandle()
	var ret []byte
	err := codec.NewEncoderBytes(&ret, h).Encode(src)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
