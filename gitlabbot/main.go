package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/keybase/managed-bots/gitlabbot/gitlabbot"

	_ "github.com/go-sql-driver/mysql"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/sync/errgroup"
)

type Options struct {
	*base.Options
	HTTPPrefix        string
	WebhookSecret     string
	OAuthClientID     string
	OAuthClientSecret string
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
		Server: base.NewServer("gitlabbot", opts.Announcement, opts.AWSOpts, opts.MultiDSN),
		opts:   opts,
	}
}

const backs = "```"

func (s *BotServer) makeAdvertisement() kbchat.Advertisement {
	subExtended := fmt.Sprintf(`Enables posting updates from the provided GitLab project to this conversation.

Example:%s
!gitlab subscribe keybase/client%s

Subscribe to a specific branch:%s
!gitlab subscribe facebook/react gh-pages%s`,
		backs, backs, backs, backs)

	unsubExtended := fmt.Sprintf(`Disables updates from the provided GitLab project to this conversation.

Example:%s
!gitlab unsubscribe keybase/client%s

Unsubscribe from a specific branch:%s
!gitlab unsubscribe facebook/react gh-pages%s`,
		backs, backs, backs, backs)

	cmds := []chat1.UserBotCommandInput{
		{
			Name:        "gitlab subscribe",
			Description: "Enable updates from GitLab projects",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!gitlab subscribe* <username/project> [branch]`,
				DesktopBody: subExtended,
				MobileBody:  subExtended,
			},
		},
		{
			Name:        "gitlab unsubscribe",
			Description: "Disable updates from GitLab projects",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!gitlab unsubscribe* <username/project> [branch]`,
				DesktopBody: unsubExtended,
				MobileBody:  unsubExtended,
			},
		},
		base.GetFeedbackCommandAdvertisement(s.kbc.GetUsername()),
	}
	return kbchat.Advertisement{
		Alias: "GitLab",
		Advertisements: []chat1.AdvertiseCommandAPIParam{
			{
				Typ:      "public",
				Commands: cmds,
			},
		},
	}
}

func (s *BotServer) getConfig() (webhookSecret string, err error) {
	if s.opts.WebhookSecret != "" {
		return s.opts.WebhookSecret, nil
	}
	path := fmt.Sprintf("/keybase/private/%s/credentials.json", s.kbc.GetUsername())
	cmd := s.opts.Command("fs", "read", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	s.Debug("Running `keybase fs read` on %q and waiting for it to finish...\n", path)
	if err := cmd.Run(); err != nil {
		return "", err
	}

	var j struct {
		WebhookSecret string `json:"webhook_secret"`
	}

	if err := json.Unmarshal(out.Bytes(), &j); err != nil {
		return "", err
	}

	return j.WebhookSecret, nil
}

func (s *BotServer) Go() (err error) {
	if s.kbc, err = s.Start(s.opts.KeybaseLocation, s.opts.Home, s.opts.ErrReportConv); err != nil {
		return err
	}

	secret, err := s.getConfig()
	if err != nil {
		s.Errorf("failed to get configuration: %s", err)
		return
	}
	sdb, err := sql.Open("mysql", s.opts.DSN)
	if err != nil {
		s.Errorf("failed to connect to MySQL: %s", err)
		return err
	}
	defer sdb.Close()
	db := gitlabbot.NewDB(sdb)
	if _, err := s.kbc.AdvertiseCommands(s.makeAdvertisement()); err != nil {
		s.Errorf("advertise error: %s", err)
		return err
	}
	if err := s.SendAnnouncement(s.opts.Announcement, "I live."); err != nil {
		s.Errorf("failed to announce self: %s", err)
	}

	debugConfig := base.NewChatDebugOutputConfig(s.kbc, s.opts.ErrReportConv)
	stats, err := base.NewStatsRegistry(debugConfig, s.opts.StathatEZKey, s.Name())
	if err != nil {
		s.Debug("unable to create stats", err)
		return err
	}
	handler := gitlabbot.NewHandler(stats, s.kbc, debugConfig, db, s.opts.HTTPPrefix, secret)
	httpSrv := gitlabbot.NewHTTPSrv(stats, s.kbc, debugConfig, db, handler, secret)
	eg := &errgroup.Group{}
	s.GoWithRecover(eg, func() error { return s.Listen(handler) })
	s.GoWithRecover(eg, httpSrv.Listen)
	s.GoWithRecover(eg, func() error { return s.HandleSignals(httpSrv) })
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
	fs.StringVar(&opts.HTTPPrefix, "http-prefix", os.Getenv("BOT_HTTP_PREFIX"), "address of bots HTTP server for webhooks")
	fs.StringVar(&opts.WebhookSecret, "secret", os.Getenv("BOT_WEBHOOK_SECRET"), "Webhook secret")
	if err := opts.Parse(fs, os.Args); err != nil {
		return 3
	}
	if len(opts.DSN) == 0 {
		fmt.Printf("must specify a database DSN\n")
		return 3
	}
	bs := NewBotServer(*opts)
	if err := bs.Go(); err != nil {
		fmt.Printf("error running chat loop: %v\n", err)
		return 3
	}
	return 0
}
