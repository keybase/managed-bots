package base

import (
	"flag"
	"os"
	"os/exec"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
)

type AWSOptions struct {
	AWSRegion          string
	CloudWatchLogGroup string
}

func (o *AWSOptions) IsEmpty() bool {
	return o == nil || (o.AWSRegion == "" && o.CloudWatchLogGroup == "")
}

type Options struct {
	KeybaseLocation string
	Home            string
	Announcement    string
	DSN             string
	AWSOpts         *AWSOptions
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) Parse(fs *flag.FlagSet, argv []string) error {
	fs.StringVar(&o.KeybaseLocation, "keybase", "keybase", "keybase command")
	fs.StringVar(&o.Home, "home", "", "Home directory")
	fs.StringVar(&o.Announcement, "announcement", os.Getenv("BOT_ANNOUNCEMENT"),
		"Where to announce we are running")
	fs.StringVar(&o.DSN, "dsn", os.Getenv("BOT_DSN"), "Bot database DSN")

	awsOpts := &AWSOptions{}
	fs.StringVar(&awsOpts.AWSRegion, "aws-region", os.Getenv("BOT_DSN"), "AWS region for cloudwatch logs, optional")
	fs.StringVar(&awsOpts.CloudWatchLogGroup, "cloudwatch-log-group", os.Getenv("BOT_CLOUDWATCH_LOG_GROUP"), "Cloudwatch log group name, optional")
	if o.AWSOpts.IsEmpty() && !awsOpts.IsEmpty() {
		o.AWSOpts = awsOpts
	}
	if err := fs.Parse(argv[1:]); err != nil {
		return err
	}
	return nil

}

func (o *Options) Command(args ...string) *exec.Cmd {
	return kbchat.RunOptions{
		KeybaseLocation: o.KeybaseLocation,
		HomeDir:         o.Home,
	}.Command(args...)
}
