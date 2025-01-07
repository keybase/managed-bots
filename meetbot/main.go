package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"

	_ "github.com/go-sql-driver/mysql"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"github.com/keybase/managed-bots/meetbot/meetbot"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/calendar/v3"
)

type Options struct {
	*base.Options
	KBFSRoot string
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
		Server: base.NewServer("meetbot", opts.Announcement, opts.AWSOpts, opts.MultiDSN, opts.ReadSelf, kbchat.RunOptions{
			KeybaseLocation: opts.KeybaseLocation,
			HomeDir:         opts.Home,
		}),
		opts: opts,
	}
}

func (s *BotServer) makeAdvertisement() kbchat.Advertisement {
	return kbchat.Advertisement{
		Alias: "Google Meet",
		Advertisements: []chat1.AdvertiseCommandAPIParam{
			{
				Typ: "public",
				Commands: []chat1.UserBotCommandInput{
					{
						Name:        "meet",
						Description: "New Google Meet",
					},
					base.GetFeedbackCommandAdvertisement(s.kbc.GetUsername()),
				},
			},
		},
	}
}

func (s *BotServer) getOAuthConfig() (config *oauth2.Config, err error) {
	defer s.Trace(&err, "getOAuthConfig")()
	if len(s.opts.KBFSRoot) == 0 {
		return nil, fmt.Errorf("envvar BOT_KBFS_ROOT must be specified")
	}
	configPath := filepath.Join(s.opts.KBFSRoot, "credentials.json")
	cmd := s.opts.Command("fs", "read", configPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	fmt.Printf("Running `keybase fs read` on %q and waiting for it to finish...\n", configPath)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("could not read credentials.json: %v", err)
	}

	// If modifying these scopes, drop the saved tokens in the db
	config, err = google.ConfigFromJSON(out.Bytes(), calendar.CalendarEventsScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}

	return config, nil
}

func (s *BotServer) Go() (err error) {
	if s.kbc, err = s.Start(s.opts.ErrReportConv); err != nil {
		return fmt.Errorf("failed to start keybase %v", err)
	}

	config, err := s.getOAuthConfig()
	if err != nil {
		return fmt.Errorf("failed to get config %v", err)
	}

	sdb, err := sql.Open("mysql", s.opts.DSN)
	if err != nil {
		s.Errorf("failed to connect to MySQL: %s", err)
		return err
	}
	defer sdb.Close()
	db := base.NewOAuthDB(sdb)
	debugConfig := base.NewChatDebugOutputConfig(s.kbc, s.opts.ErrReportConv)
	stats, err := base.NewStatsRegistry(debugConfig, s.opts.StathatEZKey)
	if err != nil {
		s.Debug("unable to create stats: %v", err)
		return err
	}
	stats = stats.SetPrefix(s.Name())
	handler := meetbot.NewHandler(stats, s.kbc, debugConfig, db, config)
	httpSrv := meetbot.NewHTTPSrv(stats, s.kbc, debugConfig, db, handler, config)
	eg := &errgroup.Group{}
	s.GoWithRecover(eg, func() error { return s.Listen(handler) })
	s.GoWithRecover(eg, httpSrv.Listen)
	s.GoWithRecover(eg, func() error { return s.HandleSignals(httpSrv, stats) })
	s.GoWithRecover(eg, func() error { return s.AnnounceAndAdvertise(s.makeAdvertisement(), "I live.") })
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
	fs.StringVar(&opts.KBFSRoot, "kbfs-root", os.Getenv("BOT_KBFS_ROOT"), "root path to bot's KBFS backed config")
	if err := opts.Parse(fs, os.Args); err != nil {
		fmt.Printf("Unable to parse options: %v\n", err)
		return 3
	}
	if len(opts.DSN) == 0 {
		fmt.Printf("must specify a database DSN\n")
		return 3
	}

	bs := NewBotServer(*opts)
	if err := bs.Go(); err != nil {
		fmt.Printf("error running chat loop: %s\n", err)
		return 3
	}
	return 0
}
