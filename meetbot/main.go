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
	base.Options
	KBFSRoot string
}

type BotServer struct {
	*base.Server

	opts Options
	kbc  *kbchat.API
}

func NewBotServer(opts Options) *BotServer {
	return &BotServer{
		Server: base.NewServer(opts.Announcement),
		opts:   opts,
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
						Description: "Get a URL for a new meet call",
					},
				},
			},
		},
	}
}

func (s *BotServer) getOAuthConfig() (*oauth2.Config, error) {
	if len(s.opts.KBFSRoot) == 0 {
		return nil, fmt.Errorf("BOT_KBFS_ROOT must be specified\n")
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
	config, err := google.ConfigFromJSON(out.Bytes(), calendar.CalendarEventsScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}

	return config, nil
}

func (s *BotServer) Go() (err error) {
	config, err := s.getOAuthConfig()
	if err != nil {
		return fmt.Errorf("failed to get config %v", err)
	}

	if s.kbc, err = s.Start(s.opts.KeybaseLocation, s.opts.Home); err != nil {
		return fmt.Errorf("failed to start keybase %v", err)
	}

	sdb, err := sql.Open("mysql", s.opts.DSN)
	if err != nil {
		s.Debug("failed to connect to MySQL: %s", err)
		return err
	}
	db := base.NewGoogleOAuthDB(sdb)
	if _, err := s.kbc.AdvertiseCommands(s.makeAdvertisement()); err != nil {
		s.Debug("advertise error: %s", err)
		return err
	}
	if err := s.SendAnnouncement(s.opts.Announcement, "I live."); err != nil {
		s.Debug("failed to announce self: %s", err)
		return err
	}

	requests := &base.OAuthRequests{}
	handler := meetbot.NewHandler(s.kbc, db, requests, config)
	httpSrv := meetbot.NewHTTPSrv(s.kbc, db, handler, requests, config)
	var eg errgroup.Group
	eg.Go(func() error { return s.Listen(handler) })
	eg.Go(httpSrv.Listen)
	eg.Go(func() error { return s.HandleSignals(httpSrv) })
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
	var opts Options
	flag.StringVar(&opts.KeybaseLocation, "keybase", "keybase", "keybase command")
	flag.StringVar(&opts.Home, "home", "", "Home directory")
	flag.StringVar(&opts.Announcement, "announcement", os.Getenv("BOT_ANNOUNCEMENT"),
		"Where to announce we are running")
	flag.StringVar(&opts.DSN, "dsn", os.Getenv("BOT_DSN"), "bot database DSN")
	flag.StringVar(&opts.KBFSRoot, "kbfs-root", os.Getenv("BOT_KBFS_ROOT"), "root path to bot's KBFS backed config")
	flag.Parse()

	bs := NewBotServer(opts)
	if err := bs.Go(); err != nil {
		fmt.Printf("error running chat loop: %s\n", err)
	}
	return 0
}
