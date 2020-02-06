package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws/defaults"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	_ "github.com/go-sql-driver/mysql"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"github.com/keybase/managed-bots/elastiwatch/elastiwatch"
	"github.com/olivere/elastic"
	"github.com/sha1sum/aws_signing_client"
	"golang.org/x/sync/errgroup"
)

type Options struct {
	*base.Options
	ESAddress   string
	Index       string
	Email       string
	SenderEmail string
	AlertConvID chat1.ConvIDStr
	EmailConvID chat1.ConvIDStr
	Team        string
}

func NewOptions() *Options {
	return &Options{
		Options: base.NewOptions(),
	}
}

type BotServer struct {
	*base.Server

	opts Options
	kbc  *kbchat.API
}

func NewBotServer(opts Options) *BotServer {
	return &BotServer{
		Server: base.NewServer("elastiwatch", opts.Announcement, opts.AWSOpts, opts.MultiDSN),
		opts:   opts,
	}
}

const backs = "```"
const back = "`"

func (s *BotServer) makeAdvertisement() kbchat.Advertisement {
	deferExtended := fmt.Sprintf(`Defer reporting on logs lines that match the givesn regular expression. Useful if there is a known error spamming emails that is not a problem

	Example:%s
		!elastiwatch defer error loading .*%s`, backs, backs)
	unDeferExtended := fmt.Sprintf(`Remove a currently active log deferral. Deferrals IDs can be found by running %s!elastiwatch list-defers%s.

	Example:%s
		!elastiwatch undefer 2%s`, back, back, backs, backs)

	cmds := []chat1.UserBotCommandInput{
		{
			Name:        "elastiwatch defer",
			Description: "Defer logs matching a regular expression",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title: `*!elastiwatch defer* <regex>
Defer logs`,
				DesktopBody: deferExtended,
				MobileBody:  deferExtended,
			},
		},
		{
			Name:        "elastiwatch list-defers",
			Description: "List active list-defers",
		},
		{
			Name:        "elastiwatch undefer",
			Description: "Remove deferral",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title: `*!elastiwatch undefer* <deferral index>
Remove deferral`,
				DesktopBody: unDeferExtended,
				MobileBody:  unDeferExtended,
			},
		},
	}
	return kbchat.Advertisement{
		Alias: "Elastiwatch",
		Advertisements: []chat1.AdvertiseCommandAPIParam{
			{
				Typ:      "teamconvs",
				Commands: cmds,
				TeamName: s.opts.Team,
			},
		},
	}
}

func (s *BotServer) Go() (err error) {
	if s.kbc, err = s.Start(s.opts.KeybaseLocation, s.opts.Home, s.opts.ErrReportConv); err != nil {
		return err
	}
	sdb, err := sql.Open("mysql", s.opts.DSN)
	if err != nil {
		s.Errorf("failed to connect to MySQL: %s", err)
		return err
	}
	defer sdb.Close()
	db := elastiwatch.NewDB(sdb)
	if _, err := s.kbc.AdvertiseCommands(s.makeAdvertisement()); err != nil {
		s.Errorf("advertise error: %s", err)
		return err
	}
	if err := s.SendAnnouncement(s.opts.Announcement, "I live."); err != nil {
		s.Errorf("failed to announce self: %s", err)
	}
	debugConfig := base.NewChatDebugOutputConfig(s.kbc, s.opts.ErrReportConv)

	s.Debug("Connect to Elasticsearch at %s", s.opts.ESAddress)
	var emailer base.Emailer
	httpClient := http.DefaultClient
	emailer = base.DummyEmailer{}
	stats, err := base.NewStatsRegistry(debugConfig, s.opts.StathatEZKey)
	if err != nil {
		s.Errorf("failed to initialize stats: %s", err)
	}
	if s.opts.AWSOpts != nil {
		s.Debug("Using AWS HTTP client: region: %s", s.opts.AWSOpts.AWSRegion)
		signer := v4.NewSigner(defaults.Get().Config.Credentials)
		aws_signing_client.SetDebugLog(log.New(os.Stdout, "httpClient", 0))
		httpClient, err = aws_signing_client.New(signer, nil, "es", s.opts.AWSOpts.AWSRegion)
		if err != nil {
			s.Errorf("failed to make http client: %s", err)
			return err
		}
		emailer = base.NewSESEmailer(s.opts.SenderEmail, s.opts.AWSOpts.AWSRegion, debugConfig)
	}
	cli, err := elastic.NewClient(
		elastic.SetURL(s.opts.ESAddress),
		elastic.SetSniff(false),
		elastic.SetHttpClient(httpClient),
	)
	if err != nil {
		s.Errorf("unable to connect to Elasticsearch: %s", err)
		return err
	}
	s.Debug("Connected to ElasticSearch")

	httpSrv := elastiwatch.NewHTTPSrv(stats, s.kbc, debugConfig, db)
	handler := elastiwatch.NewHandler(s.kbc, debugConfig, httpSrv, db)
	logwatch := elastiwatch.NewLogWatch(cli, db, s.opts.Index, s.opts.Email, emailer, s.opts.AlertConvID,
		s.opts.EmailConvID, debugConfig)
	eg := &errgroup.Group{}
	s.GoWithRecover(eg, func() error { return s.Listen(handler) })
	s.GoWithRecover(eg, httpSrv.Listen)
	s.GoWithRecover(eg, func() error { return s.HandleSignals(httpSrv, logwatch) })
	s.GoWithRecover(eg, func() error { return logwatch.Run() })
	if err := eg.Wait(); err != nil {
		s.Debug("wait error: %s", err)
		return err
	}
	return nil
}

func main() {
	rc := mainInner()
	os.Exit(rc)
}

func mainInner() int {
	opts := NewOptions()
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	var alertConvID, emailConvID string
	fs.StringVar(&opts.ESAddress, "esaddress", os.Getenv("ESADDRESS"), "Elasticsearch address")
	fs.StringVar(&opts.Index, "index", os.Getenv("INDEX"), "Elasticsearch index")
	fs.StringVar(&opts.Email, "email", os.Getenv("EMAIL"), "Destination email address")
	fs.StringVar(&opts.SenderEmail, "sender-email", os.Getenv("SENDER_EMAIL"), "Sourceemail address")
	fs.StringVar(&opts.Team, "team", os.Getenv("TEAM"), "Team")
	fs.StringVar(&alertConvID, "alert-convid", os.Getenv("ALERT_CONVID"), "Alerting conv id")
	fs.StringVar(&emailConvID, "email-convid", os.Getenv("EMAIL_CONVID"), "Email conv id")

	if err := opts.Parse(fs, os.Args); err != nil {
		fmt.Printf("Unable to parse options: %v\n", err)
		return 3
	}
	if len(opts.DSN) == 0 {
		fmt.Printf("must specify a database DSN\n")
		return 3
	}
	if len(opts.ESAddress) == 0 {
		fmt.Printf("must specify a ElasticSearch address\n")
		return 3
	}
	if len(opts.Index) == 0 {
		fmt.Printf("must specify a ElasicSearch index\n")
		return 3
	}
	if len(opts.Team) == 0 {
		fmt.Printf("must specify a team to operate in\n")
		return 3
	}
	opts.AlertConvID = chat1.ConvIDStr(alertConvID)
	opts.EmailConvID = chat1.ConvIDStr(emailConvID)
	bs := NewBotServer(*opts)
	if err := bs.Go(); err != nil {
		fmt.Printf("error running chat loop: %v\n", err)
		return 3
	}
	return 0
}
