package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"github.com/keybase/managed-bots/webhookbot/webhookbot"
	"golang.org/x/sync/errgroup"
)

type Options struct {
	KeybaseLocation string
	Home            string
	Announcement    string
	HTTPPrefix      string
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
	createExtended := fmt.Sprintf(`Create a new webhook for sending messages into the current conversation. You must supply a name as well to identify the webhook. To use a webhook URL, supply a %smsg%s URL parameter, or a JSON POST body with a field %smsg%s.

	Example:%s
		!webhook create alerts%s`, back, back, back, back, backs, backs)
	removeExtended := fmt.Sprintf(`Remove a webhook from the current conversation. You must supply the name of the webhook.

	Example:%s
		!webhook remove alerts%s`, backs, backs)

	cmds := []chat1.UserBotCommandInput{
		{
			Name:        "webhook create",
			Description: "Create a new webhook for sending into the current conversation",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title: `*!webhook create* <name>
Create a webhook`,
				DesktopBody: createExtended,
				MobileBody:  createExtended,
			},
		},
		{
			Name:        "webhook list",
			Description: "List active webhooks in the current conversation",
		},
		{
			Name:        "webhook remove",
			Description: "Remove a webhook from the current conversation",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title: `*!webhook remove* <name>
Remove a webhook`,
				DesktopBody: removeExtended,
				MobileBody:  removeExtended,
			},
		},
	}
	return kbchat.Advertisement{
		Alias: "Webhooks",
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
	db := webhookbot.NewDB(sdb)
	if _, err := s.kbc.AdvertiseCommands(s.makeAdvertisement()); err != nil {
		s.Debug("advertise error: %s", err)
		return err
	}
	if err := s.SendAnnouncement(s.opts.Announcement, "I live."); err != nil {
		s.Debug("failed to announce self: %s", err)
		return err
	}

	httpSrv := webhookbot.NewHTTPSrv(s.kbc, db)
	handler := webhookbot.NewHandler(s.kbc, httpSrv, db, s.opts.HTTPPrefix)
	var eg errgroup.Group
	eg.Go(handler.Listen)
	eg.Go(httpSrv.Listen)
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
	flag.StringVar(&opts.HTTPPrefix, "http-prefix", os.Getenv("BOT_HTTP_PREFIX"), "")
	flag.StringVar(&opts.DSN, "dsn", os.Getenv("BOT_DSN"), "Poll database DSN")
	flag.Parse()
	if len(opts.DSN) == 0 {
		fmt.Printf("must specify a poll database DSN\n")
		return 3
	}
	bs := NewBotServer(opts)
	if err := bs.Go(); err != nil {
		fmt.Printf("error running chat loop: %s\n", err.Error())
	}
	return 0
}
