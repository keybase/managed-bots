package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"

	_ "github.com/go-sql-driver/mysql"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"github.com/keybase/managed-bots/zoombot/zoombot"
	"golang.org/x/sync/errgroup"
)

type Options struct {
	*base.Options
	KBFSRoot          string
	HTTPPrefix        string
	OAuthClientID     string
	OAuthClientSecret string
	VerificationToken string
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
		Server: base.NewServer("zoombot", opts.Announcement, opts.AWSOpts, opts.MultiDSN, kbchat.RunOptions{
			KeybaseLocation: opts.KeybaseLocation,
			HomeDir:         opts.Home,
		}),
		opts: opts,
	}
}

func (s *BotServer) makeAdvertisement() kbchat.Advertisement {
	return kbchat.Advertisement{
		Alias: "Zoom",
		Advertisements: []chat1.AdvertiseCommandAPIParam{
			{
				Typ: "public",
				Commands: []chat1.UserBotCommandInput{
					{
						Name:        "zoom",
						Description: "New Zoom meeting",
					},
					base.GetFeedbackCommandAdvertisement(s.kbc.GetUsername()),
				},
			},
		},
	}
}

func (s *BotServer) getCredentials() (credentials *zoombot.Credentials, err error) {
	if s.opts.OAuthClientID != "" && s.opts.OAuthClientSecret != "" && s.opts.VerificationToken != "" {
		credentials = &zoombot.Credentials{
			ClientID:          s.opts.OAuthClientID,
			ClientSecret:      s.opts.OAuthClientSecret,
			VerificationToken: s.opts.VerificationToken,
		}
	} else {
		if len(s.opts.KBFSRoot) == 0 {
			return nil, fmt.Errorf("BOT_KBFS_ROOT must be specified\n")
		}
		configPath := filepath.Join(s.opts.KBFSRoot, "credentials.json")
		cmd := s.opts.Command("fs", "read", configPath)
		var out bytes.Buffer
		cmd.Stdout = &out
		s.Debug("Running `keybase fs read` on %q and waiting for it to finish...\n", configPath)
		if err := cmd.Run(); err != nil {
			return nil, err
		}

		credentials = &zoombot.Credentials{}
		if err := json.Unmarshal(out.Bytes(), credentials); err != nil {
			return nil, err
		}
	}

	if len(credentials.ClientID) == 0 || len(credentials.ClientSecret) == 0 || len(credentials.VerificationToken) == 0 {
		return nil, fmt.Errorf("must provide a clientID (len: %d), clientSecret (len: %d) and verificationToken (len: %d)",
			len(credentials.ClientID), len(credentials.ClientSecret), len(credentials.VerificationToken))
	}

	return credentials, nil
}

func (s *BotServer) Go() (err error) {
	if s.kbc, err = s.Start(s.opts.ErrReportConv); err != nil {
		return fmt.Errorf("failed to start keybase %v", err)
	}

	credentials, err := s.getCredentials()
	if err != nil {
		return fmt.Errorf("failed to get credentials %v", err)
	}

	config := &oauth2.Config{
		ClientID:     credentials.ClientID,
		ClientSecret: credentials.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://zoom.us/oauth/authorize",
			TokenURL: "https://zoom.us/oauth/token",
		},
		RedirectURL: fmt.Sprintf("%s/zoombot/oauth", s.opts.HTTPPrefix),
		Scopes:      []string{"user:read", "meeting:write"},
	}

	sdb, err := sql.Open("mysql", s.opts.DSN)
	if err != nil {
		s.Errorf("failed to connect to MySQL: %s", err)
		return err
	}
	defer sdb.Close()
	db := zoombot.NewDB(sdb)
	if _, err := s.kbc.AdvertiseCommands(s.makeAdvertisement()); err != nil {
		s.Errorf("advertise error: %s", err)
		return err
	}
	if err := s.SendAnnouncement(s.opts.Announcement, "I live."); err != nil {
		s.Errorf("failed to announce self: %s", err)
	}

	debugConfig := base.NewChatDebugOutputConfig(s.kbc, s.opts.ErrReportConv)
	stats, err := base.NewStatsRegistry(debugConfig, s.opts.StathatEZKey)
	if err != nil {
		s.Debug("unable to create stats", err)
		return err
	}
	stats = stats.SetPrefix(s.Name())
	handler := zoombot.NewHandler(stats, s.kbc, debugConfig, db, config)
	httpSrv := zoombot.NewHTTPSrv(stats, s.kbc, debugConfig, db, handler, config, credentials)
	eg := &errgroup.Group{}
	s.GoWithRecover(eg, func() error { return s.Listen(handler) })
	s.GoWithRecover(eg, httpSrv.Listen)
	s.GoWithRecover(eg, func() error { return s.HandleSignals(httpSrv, stats) })
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
	fs.StringVar(&opts.HTTPPrefix, "http-prefix", os.Getenv("BOT_HTTP_PREFIX"), "address of bots HTTP server for webhooks")
	fs.StringVar(&opts.OAuthClientID, "client-id", os.Getenv("BOT_OAUTH_CLIENT_ID"), "Zoom OAuth2 client ID")
	fs.StringVar(&opts.OAuthClientSecret, "client-secret", os.Getenv("BOT_OAUTH_CLIENT_SECRET"), "Zoom OAuth2 client secret")
	fs.StringVar(&opts.VerificationToken, "verification-token", os.Getenv("BOT_VERIFICATION_TOKEN"), "Zoom verification token")
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
