package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/keybase/managed-bots/gcalbot/gcalbot/reminderscheduler"

	"golang.org/x/oauth2"

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
	*base.Options
	KBFSRoot   string
	HTTPPrefix string
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
		Server: base.NewServer(opts.Announcement, opts.AWSOpts),
		opts:   opts,
	}
}

const back = "`"
const backs = "```"

func (s *BotServer) makeAdvertisement() kbchat.Advertisement {

	accountsConnectDesc := fmt.Sprintf(`Connects a Google account to the Google Calendar bot and stores the connection under a descriptive nickname.
View your connected Google accounts using %s!gcal accounts list%s

Examples:%s
!gcal accounts connect personal
!gcal accounts connect work%s`,
		back, back, backs, backs)

	accountsDisconnectDesc := fmt.Sprintf(`Disconnects a Google account from the Google Calendar bot given the connection's nickname.
View your connected Google accounts using %s!gcal accounts list%s

Examples:%s
!gcal accounts disconnect personal
!gcal accounts disconnect work%s`,
		back, back, backs, backs)

	calendarsListDesc := fmt.Sprintf(`Lists calendars associated with a Google account given the account connection's nickname.
View your connected Google accounts using %s!gcal accounts list%s

Examples:%s
!gcal calendars list personal
!gcal calendars list work%s`,
		back, back, backs, backs)

	invitesSubscribeDesc := fmt.Sprintf(`Subscribes to event invites over direct message for the primary calendar of a Google account given the account connection's nickname.
View your connected Google accounts using %s!gcal accounts list%s

Examples:%s
!gcal invites subscribe personal
!gcal invites subscribe work%s`,
		back, back, backs, backs)

	invitesUnsubscribeDesc := fmt.Sprintf(`Unsubscribes from event invites for the primary calendar of a Google account given the account connection's nickname.
View your connected Google accounts using %s!gcal accounts list%s

Examples:%s
!gcal invites unsubscribe personal
!gcal invites unsubscribe work%s`,
		back, back, backs, backs)

	remindersSubscribeDesc := fmt.Sprintf(`Subscribes to event reminders over direct message for the primary calendar of a Google account given the account connection's nickname.
Reminders can be set for 0 minutes up to 60 minutes before the start of an event.
You can configure multiple event reminders per account.
View your connected Google accounts using %s!gcal accounts list%s
View existing reminder configurations using %s!gcal reminders list%s

Examples:%s
!gcal reminders subscribe personal 0
!gcal reminders subscribe family 5
!gcal reminders subscribe work 60%s`,
		back, back, back, back, backs, backs)

	remindersUnsubscribeDesc := fmt.Sprintf(`Unsubscribes from event reminders for the primary calendar of a Google account given the account connection's nickname.
View your connected Google accounts using %s!gcal accounts list%s
View existing reminder configurations using %s!gcal reminders list%s

Examples:%s
!gcal reminders unsubscribe personal 0
!gcal reminders unsubscribe family 5
!gcal reminders unsubscribe work 60%s`,
		back, back, back, back, backs, backs)

	remindersListDesc := fmt.Sprintf(`Lists event reminders configured for the primary calendar of a Google account given the account connection's nickname.
View your connected Google accounts using %s!gcal accounts list%s

Examples:%s
!gcal reminders list personal
!gcal reminders list work%s`,
		back, back, backs, backs)

	commands := []chat1.UserBotCommandInput{
		{
			Name:        "gcal accounts list",
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

		{
			Name:        "gcal calendars list",
			Description: "List calendars that a Google account is subscribed to",
			Usage:       "<account nickname>",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       "*!gcal calendars list* <account nickname>",
				DesktopBody: calendarsListDesc,
				MobileBody:  calendarsListDesc,
			},
		},

		{
			Name:        "gcal invites subscribe",
			Description: "Subscribe to event invites over direct message for your primary calendar",
			Usage:       "<account nickname>",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       "*!gcal invites subscribe* <account nickname>",
				DesktopBody: invitesSubscribeDesc,
				MobileBody:  invitesSubscribeDesc,
			},
		},
		{
			Name:        "gcal invites unsubscribe",
			Description: "Unsubscribe from event invites for your primary calendar",
			Usage:       "<account nickname>",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       "*!gcal invites unsubscribe* <account nickname>",
				DesktopBody: invitesUnsubscribeDesc,
				MobileBody:  invitesUnsubscribeDesc,
			},
		},

		{
			Name:        "gcal reminders subscribe",
			Description: "Subscribe to event reminders over direct message for your primary calendar",
			Usage:       "<account nickname> <minutes before start of event>",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       "*!gcal reminders subscribe* <account nickname> <minutes before start of event>",
				DesktopBody: remindersSubscribeDesc,
				MobileBody:  remindersSubscribeDesc,
			},
		},
		{
			Name:        "gcal reminders unsubscribe",
			Description: "Unsubscribe from event reminders for your primary calendar",
			Usage:       "<account nickname> <minutes before start of event>",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       "*!gcal reminders unsubscribe* <account nickname> <minutes before start of event>",
				DesktopBody: remindersUnsubscribeDesc,
				MobileBody:  remindersUnsubscribeDesc,
			},
		},
		{
			Name:        "gcal reminders list",
			Description: "List event reminder configurations for your primary calendar",
			Usage:       "<account nickname>",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       "*!gcal reminders list* <account nickname>",
				DesktopBody: remindersListDesc,
				MobileBody:  remindersListDesc,
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
	// Need CalendarReadonlyScope to list calendars and get primary calendar
	// Need CalendarEventsScope to set a response status for events that a user is invited to
	config, err := google.ConfigFromJSON(out.Bytes(), calendar.CalendarReadonlyScope, calendar.CalendarEventsScope)
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

	if s.kbc, err = s.Start(s.opts.KeybaseLocation, s.opts.Home, s.opts.ErrReportConv); err != nil {
		return fmt.Errorf("failed to start keybase %v", err)
	}

	sdb, err := sql.Open("mysql", s.opts.DSN)
	if err != nil {
		s.Errorf("failed to connect to MySQL: %s", err)
		return err
	}
	defer sdb.Close()
	db := gcalbot.NewDB(sdb)
	if _, err := s.kbc.AdvertiseCommands(s.makeAdvertisement()); err != nil {
		s.Errorf("advertise error: %s", err)
		return err
	}
	if err := s.SendAnnouncement(s.opts.Announcement, "I live."); err != nil {
		s.Errorf("failed to announce self: %s", err)
	}

	requests := &base.OAuthRequests{}
	debugConfig := base.NewChatDebugOutputConfig(s.kbc, s.opts.ErrReportConv)
	handler := gcalbot.NewHandler(s.kbc, debugConfig, db, requests, config, s.opts.HTTPPrefix)
	httpSrv := gcalbot.NewHTTPSrv(s.kbc, debugConfig, db, handler, requests, config)
	renewScheduler := gcalbot.NewRenewChannelScheduler(debugConfig, db, config, s.opts.HTTPPrefix)
	reminderScheduler := reminderscheduler.NewReminderScheduler(debugConfig, db, config)
	var eg errgroup.Group
	eg.Go(func() error { return s.Listen(handler) })
	eg.Go(httpSrv.Listen)
	eg.Go(renewScheduler.Run)
	eg.Go(reminderScheduler.Run)
	eg.Go(func() error { return s.HandleSignals(httpSrv, renewScheduler, reminderScheduler) })
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
	fs.StringVar(&opts.HTTPPrefix, "http-prefix", os.Getenv("BOT_HTTP_PREFIX"), "address of the bot's web server")
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
