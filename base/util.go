package base

import (
	"encoding/base64"

	"github.com/keybase/go-codec/codec"
)

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

func ShortConvID(convID string) string {
	if len(convID) <= 20 {
		return convID
	}
	return convID[:20]
}

func URLEncoder() *base64.Encoding {
	return base64.URLEncoding.WithPadding(base64.NoPadding)
}
