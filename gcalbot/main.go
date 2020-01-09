package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"github.com/keybase/managed-bots/gcalbot/gcalbot"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/calendar/v3"
)

type Options struct {
	KeybaseLocation string
	Home            string
	Announcement    string
	DSN             string
	KBFSRoot        string
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

const backs = "```"

func (s *BotServer) makeAdvertisement() kbchat.Advertisement {

	accountsConnectDesc := fmt.Sprintf(`Connects a Google account to the Google Calendar bot and stores the connection under a descriptive nickname.

Examples:%s
!gcal accounts connect personal
!gcal accounts connect work%s`,
		backs, backs)

	accountsDisconnectDesc := fmt.Sprintf(`Disconnects a Google account from the Google Calendar bot given the connection's nickname.

Examples:%s
!gcal accounts disconnect personal
!gcal accounts disconnect work%s`,
		backs, backs)

	commands := []chat1.UserBotCommandInput{
		{
			Name:        "gcal accounts",
			Description: "List your connected Google accounts",
		},
		{
			Name:        "gcal accounts connect",
			Description: "Connect a Google account",
			Usage:       "<account nickname>",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!gcal accounts connect* <account nickname>`,
				DesktopBody: accountsConnectDesc,
				MobileBody:  accountsConnectDesc,
			},
		},
		{
			Name:        "gcal accounts disconnect",
			Description: "Disconnect a Google account",
			Usage:       "<account nickname>",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!gcal accounts disconnect* <account nickname>`,
				DesktopBody: accountsDisconnectDesc,
				MobileBody:  accountsDisconnectDesc,
			},
		},
	}

	return kbchat.Advertisement{
		Alias: "Google Calendar",
		Advertisements: []chat1.AdvertiseCommandAPIParam{
			{
				Typ:      "public",
				Commands: commands,
			},
		},
	}
}

func (s *BotServer) Go() (err error) {
	if len(s.opts.KBFSRoot) == 0 {
		return fmt.Errorf("BOT_KBFS_ROOT must be specified\n")
	}
	configPath := filepath.Join(s.opts.KBFSRoot, "credentials.json")
	cmd := exec.Command(s.opts.KeybaseLocation, "fs", "read", configPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	fmt.Printf("Running `keybase fs read` on %q and waiting for it to finish...\n", configPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not read credentials.json: %v", err)
	}

	// If modifying these scopes, drop the saved tokens in the db
	config, err := google.ConfigFromJSON(out.Bytes(), calendar.CalendarReadonlyScope)
	if err != nil {
		return fmt.Errorf("unable to parse client secret file to config: %v", err)
	}

	if s.kbc, err = s.Start(s.opts.KeybaseLocation, s.opts.Home); err != nil {
		return fmt.Errorf("failed to start keybase %v", err)
	}

	sdb, err := sql.Open("mysql", s.opts.DSN)
	if err != nil {
		s.Debug("failed to connect to MySQL: %s", err)
		return err
	}
	db := gcalbot.NewDB(sdb)
	if _, err := s.kbc.AdvertiseCommands(s.makeAdvertisement()); err != nil {
		s.Debug("advertise error: %s", err)
		return err
	}
	if err := s.SendAnnouncement(s.opts.Announcement, "I live."); err != nil {
		s.Debug("failed to announce self: %s", err)
		return err
	}

	requests := &base.OAuthRequests{}
	handler := gcalbot.NewHandler(s.kbc, db, requests, config)
	httpSrv := gcalbot.NewHTTPSrv(s.kbc, db, handler, requests, config)
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
