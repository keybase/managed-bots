package main

import (
	"database/sql"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"github.com/keybase/managed-bots/triviabot/triviabot"
	"golang.org/x/sync/errgroup"
)

type BotServer struct {
	*base.Server

	opts base.Options
	kbc  *kbchat.API
}

func NewBotServer(opts base.Options) *BotServer {
	return &BotServer{
		Server: base.NewServer("triviabot", opts.Announcement, opts.AWSOpts, opts.MultiDSN, opts.ReadSelf, kbchat.RunOptions{
			KeybaseLocation: opts.KeybaseLocation,
			HomeDir:         opts.Home,
		}),
		opts: opts,
	}
}

func (s *BotServer) makeAdvertisement() kbchat.Advertisement {
	cmds := []chat1.UserBotCommandInput{
		{
			Name:        "trivia begin",
			Description: "Begin a new question asking session",
		},
		{
			Name:        "trivia end",
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
		base.GetFeedbackCommandAdvertisement(s.kbc.GetUsername()),
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
	if s.kbc, err = s.Start(s.opts.ErrReportConv); err != nil {
		return err
	}
	sdb, err := sql.Open("mysql", s.opts.DSN)
	if err != nil {
		s.Errorf("failed to connect to MySQL: %s", err)
		return err
	}
	defer sdb.Close()
	db := triviabot.NewDB(sdb)

	debugConfig := base.NewChatDebugOutputConfig(s.kbc, s.opts.ErrReportConv)
	stats, err := base.NewStatsRegistry(debugConfig, s.opts.StathatEZKey)
	if err != nil {
		s.Debug("unable to create stats: %v", err)
		return err
	}
	stats = stats.SetPrefix(s.Name())
	handler := triviabot.NewHandler(stats, s.kbc, debugConfig, db)
	eg := &errgroup.Group{}
	s.GoWithRecover(eg, func() error { return s.Listen(handler) })
	s.GoWithRecover(eg, func() error { return s.HandleSignals(stats) })
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
	rand.Seed(time.Now().Unix())

	opts := base.NewOptions()
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
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
