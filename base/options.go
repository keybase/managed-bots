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
	// Location of the keybase binary
	KeybaseLocation string
	// Home directory for keybase service
	Home string
	// Conversation name or ID to announce when the bot begins
	Announcement string
	// Conversation name or ID to report bot errors to
	ErrReportConv string
	// Database Source Name
	DSN          string
	MultiDSN     string
	StathatEZKey string
	// Allow the bot to read it's own messages (default: false)
	ReadSelf bool
	AWSOpts  *AWSOptions
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) Parse(fs *flag.FlagSet, argv []string) error {
	fs.StringVar(&o.KeybaseLocation, "keybase", "keybase", "keybase command")
	fs.StringVar(&o.Home, "home", "", "Home directory")
	fs.StringVar(&o.Announcement, "announcement", os.Getenv("BOT_ANNOUNCEMENT"),
		"Conversation name or ID to announce we are running")
	fs.StringVar(&o.ErrReportConv, "err-report-conv", os.Getenv("BOT_ERR_REPORT_CONV"),
		"Conversation name or ID to report errors to")
	fs.StringVar(&o.DSN, "dsn", os.Getenv("BOT_DSN"), "Bot database DSN")
	fs.StringVar(&o.MultiDSN, "multi-dsn", os.Getenv("BOT_MULTI_DSN"), "Bot multi coordination database DSN")
	fs.StringVar(&o.StathatEZKey, "stathat-ezkey", os.Getenv("BOT_STATHAT_EZKEY"), "Bot stathat ezkey")
	fs.BoolVar(&o.ReadSelf, "read-self", false, "Allow the bot to read it's own messages")

	awsOpts := &AWSOptions{}
	fs.StringVar(&awsOpts.AWSRegion, "aws-region", os.Getenv("BOT_AWS_REGION"), "AWS region for cloudwatch logs, optional")
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
