package base

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-sql-driver/mysql"
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
	DSN      string
	MultiDSN string
	// Allow the bot to read it's own messages (default: false)
	ReadSelf bool
	AWSOpts  *AWSOptions

	// Optional, used for debugging to identify the bot.
	DebugTag string

	ShowtrendsAddr string
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) Parse(fs *flag.FlagSet, argv []string) error {
	var mysqlTLSCA string
	fs.StringVar(&o.KeybaseLocation, "keybase", "keybase", "keybase command")
	fs.StringVar(&o.Home, "home", "", "Home directory")
	fs.StringVar(&o.Announcement, "announcement", os.Getenv("BOT_ANNOUNCEMENT"),
		"Conversation name or ID to announce we are running")
	fs.StringVar(&o.ErrReportConv, "err-report-conv", os.Getenv("BOT_ERR_REPORT_CONV"),
		"Conversation name or ID to report errors to")
	fs.StringVar(&o.DSN, "dsn", os.Getenv("BOT_DSN"), "Bot database DSN")
	fs.StringVar(&o.MultiDSN, "multi-dsn", os.Getenv("BOT_MULTI_DSN"), "Bot multi coordination database DSN")
	fs.StringVar(&mysqlTLSCA, "mysql-tls-ca", os.Getenv("BOT_MYSQL_TLS_CA"), "Bot MySQL TLS CA")
	fs.BoolVar(&o.ReadSelf, "read-self", false, "Allow the bot to read it's own messages")
	fs.StringVar(&o.DebugTag, "debug-tag", os.Getenv("BOT_DEBUG_TAG"), "Bot debug tag")
	fs.StringVar(&o.ShowtrendsAddr, "showtrends-addr", os.Getenv("BOT_SHOWTRENDS_ADDR"), "showtrends server address for stats")

	awsOpts := &AWSOptions{}
	fs.StringVar(&awsOpts.AWSRegion, "aws-region", os.Getenv("BOT_AWS_REGION"), "AWS region for cloudwatch logs, optional")
	fs.StringVar(&awsOpts.CloudWatchLogGroup, "cloudwatch-log-group", os.Getenv("BOT_CLOUDWATCH_LOG_GROUP"), "Cloudwatch log group name, optional")
	if o.AWSOpts.IsEmpty() && !awsOpts.IsEmpty() {
		o.AWSOpts = awsOpts
	}
	if err := fs.Parse(argv[1:]); err != nil {
		return err
	}

	// configure TLS CAs if specified
	if mysqlTLSCA != "" {
		if err := o.configureMySQLTLS(mysqlTLSCA); err != nil {
			return err
		}
	}
	return nil
}

func (o *Options) addTLSToDSN(dsn string, tlsConfigName string) string {
	if strings.Contains(dsn, "?") {
		return dsn + "&tls=" + tlsConfigName
	}
	return dsn + "?tls=" + tlsConfigName
}

func (o *Options) configureMySQLTLS(mysqlTLSCA string) error {
	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM([]byte(mysqlTLSCA)) {
		return errors.New("unable to load MySQL TLS CAs")
	}
	tlsConfig := &tls.Config{
		RootCAs:    rootCAs,
		MinVersion: tls.VersionTLS12,
	}
	configName := "bot-mysql-tls"
	if err := mysql.RegisterTLSConfig(configName, tlsConfig); err != nil {
		return fmt.Errorf("error registering MySQL TLS config: %s", err)
	}

	o.DSN = o.addTLSToDSN(o.DSN, configName)
	o.MultiDSN = o.addTLSToDSN(o.MultiDSN, configName)

	return nil
}

func (o *Options) Command(args ...string) *exec.Cmd {
	return kbchat.RunOptions{
		KeybaseLocation: o.KeybaseLocation,
		HomeDir:         o.Home,
		DebugTag:        o.DebugTag,
	}.Command(args...)
}
