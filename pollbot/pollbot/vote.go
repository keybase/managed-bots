package pollbot

import (
	"fmt"

	"github.com/keybase/managed-bots/base"
)

type Vote struct {
	ID     string
	Choice int
}

type voteToEncode struct {
	ID     string `codec:"d"`
	Choice int    `codec:"i"`
}

func NewVote(id string, choice int) Vote {
	return Vote{
		ID:     id,
		Choice: choice,
	}
}

func NewVoteFromEncoded(sdat string) (Vote, error) {
	var ve voteToEncode
	dat, err := base.URLEncoder().DecodeString(sdat)
	if err != nil {
		return Vote{}, err
	}
	if err := base.MsgpackDecode(&ve, dat); err != nil {
		return Vote{}, err
	}
	if ve.ID == "" {
		return Vote{}, fmt.Errorf("missing poll id")
	}
	return Vote(ve), nil
}

func (v Vote) Encode() string {
	mdat, _ := base.MsgpackEncode(voteToEncode(v))
	return base.URLEncoder().EncodeToString(mdat)
}
