package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"github.com/keybase/managed-bots/triviabot/triviabot"
	"golang.org/x/sync/errgroup"
)

type Options struct {
	KeybaseLocation string
	Home            string
	Announcement    string
	DSN             string
}

func newOptions() Options {
	return Options{}
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

const back = "`"
const backs = "```"

func (s *BotServer) makeAdvertisement() kbchat.Advertisement {
	cmds := []chat1.UserBotCommandInput{
		{
			Name:        "trivia start",
			Description: "Start a new question asking session",
		},
		{
			Name:        "trivia stop",
			Description: "End the current question asking session",
		},
		{
			Name:        "trivia top",
			Description: "Show the top users for this conversation",
		},
		{
			Name:        "trivia reset",
			Description: "Reset the scores leaderboard",
		},
	}
	return kbchat.Advertisement{
		Alias: "Trivia",
		Advertisements: []chat1.AdvertiseCommandAPIParam{
			{
				Typ:      "public",
				Commands: cmds,
			},
		},
	}
}

func (s *BotServer) Go() (err error) {
	if s.kbc, err = s.Start(s.opts.KeybaseLocation, s.opts.Home); err != nil {
		return err
	}
	sdb, err := sql.Open("mysql", s.opts.DSN)
	if err != nil {
		s.Debug("failed to connect to MySQL: %s", err)
		return err
	}
	db := triviabot.NewDB(sdb)
	if _, err := s.kbc.AdvertiseCommands(s.makeAdvertisement()); err != nil {
		s.Debug("advertise error: %s", err)
		return err
	}
	if err := s.SendAnnouncement(s.opts.Announcement, "I live."); err != nil {
		s.Debug("failed to announce self: %s", err)
		return err
	}

	handler := triviabot.NewHandler(s.kbc, db)
	var eg errgroup.Group
	eg.Go(handler.Listen)
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
	opts := newOptions()

	flag.StringVar(&opts.KeybaseLocation, "keybase", "keybase", "keybase command")
	flag.StringVar(&opts.Home, "home", "", "Home directory")
	flag.StringVar(&opts.Announcement, "announcement", os.Getenv("BOT_ANNOUNCEMENT"),
		"Where to announce we are running")
	flag.StringVar(&opts.DSN, "dsn", os.Getenv("BOT_DSN"), "Poll database DSN")
	flag.Parse()
	if len(opts.DSN) == 0 {
		fmt.Printf("must specify a poll database DSN\n")
		return 3
	}
	bs := NewBotServer(opts)
	if err := bs.Go(); err != nil {
		fmt.Printf("error running chat loop: %s\n", err)
	}
	return 0
}
