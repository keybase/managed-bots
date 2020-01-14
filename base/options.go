package base

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
)

type Options struct {
	KeybaseLocation string
	Home            string
	Announcement    string
	DSN             string
}

func (o *Options) Parse(fs *flag.FlagSet, argv []string) error {
	if len(argv) <= 1 {
		return fmt.Errorf("Bad usage: no arguments specified")
	}
	fs.StringVar(&o.KeybaseLocation, "keybase", "keybase", "keybase command")
	fs.StringVar(&o.Home, "home", "", "Home directory")
	fs.StringVar(&o.Announcement, "announcement", os.Getenv("BOT_ANNOUNCEMENT"),
		"Where to announce we are running")
	fs.StringVar(&o.DSN, "dsn", os.Getenv("BOT_DSN"), "Bot database DSN")
	if err := fs.Parse(argv[1:]); err != nil {
		return err
	}
	if len(o.DSN) == 0 {
		return fmt.Errorf("must specify a database DSN\n")
	}
	return nil

}

func (o *Options) Command(args ...string) *exec.Cmd {
	return kbchat.RunOptions{
		KeybaseLocation: o.KeybaseLocation,
		HomeDir:         o.Home,
	}.Command(args...)
}
