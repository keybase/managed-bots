package base

import (
	"os/exec"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
)

type Options struct {
	KeybaseLocation string
	Home            string
	Announcement    string
	DSN             string
}

func (o Options) Command(args ...string) *exec.Cmd {
	return kbchat.RunOptions{
		KeybaseLocation: o.KeybaseLocation,
		HomeDir:         o.Home,
	}.Command(args...)
}
