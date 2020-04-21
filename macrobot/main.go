package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/keybase/managed-bots/macrobot/macrobot"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/sync/errgroup"
)

type BotServer struct {
	*base.Server

	opts base.Options
	kbc  *kbchat.API
}

func NewBotServer(opts base.Options) *BotServer {
	return &BotServer{
		Server: base.NewServer("macrobot", opts.Announcement, opts.AWSOpts, opts.MultiDSN, opts.ReadSelf, kbchat.RunOptions{
			KeybaseLocation: opts.KeybaseLocation,
			HomeDir:         opts.Home,
		}),
		opts: opts,
	}
}

const (
	back       = "`"
	backs      = "```"
	createHelp = `You must specify a name for the macro, such as 'docs' or 'lunchflip' as well as a message for the bot to send whenever you invoke the macro.

Examples:%s
!macro create docs 'You can find documentation at: https://keybase.io/docs'
!macro create lunchflip '/flip alice, bob, charlie'%s
You can run the above macros using %s!docs%s or %s!lunchflip%s`
)

func (s *BotServer) makeAdvertisement() kbchat.Advertisement {
	createDesc := fmt.Sprintf("Create a new macro for the current team or conversation. %s",
		createHelp, backs, backs, back, back, back, back)
	createForChannelDesc := fmt.Sprintf("Create a new macro for the current channel. %s",
		createHelp, back, back, backs, backs)
	removeDesc := fmt.Sprintf(`Remove a macro from the current team or conversation. You must specify the name of the macro.

Examples:%s
!macro remove docs
!macro remove lunchflip%s`,
		backs, backs)

	cmds := []chat1.UserBotCommandInput{
		{
			Name:        "macro create",
			Description: "Create a new macro for the current team or conversation",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!macro create* <name> <message>`,
				DesktopBody: createDesc,
				MobileBody:  createDesc,
			},
		},
		{
			Name:        "macro create-for-channel",
			Description: "Create a new macro for the current channel",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!macro create-for-channel* <name> <message>`,
				DesktopBody: createForChannelDesc,
				MobileBody:  createForChannelDesc,
			},
		},
		{
			Name:        "macro list",
			Description: "List available macros for the current team or conversation",
		},
		{
			Name:        "macro remove",
			Description: "Remove a macro from the current team or conversation",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!macro remove* <name>`,
				DesktopBody: removeDesc,
				MobileBody:  removeDesc,
			},
		},
		base.GetFeedbackCommandAdvertisement(s.kbc.GetUsername()),
	}
	return kbchat.Advertisement{
		Alias: "Macros",
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
	db := macrobot.NewDB(sdb)

	debugConfig := base.NewChatDebugOutputConfig(s.kbc, s.opts.ErrReportConv)
	stats, err := base.NewStatsRegistry(debugConfig, s.opts.StathatEZKey)
	if err != nil {
		s.Debug("unable to create stats: %v", err)
		return err
	}
	stats = stats.SetPrefix(s.Name())
	handler := macrobot.NewHandler(stats, s.kbc, debugConfig, db)
	httpSrv := macrobot.NewHTTPSrv(stats, debugConfig)
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
