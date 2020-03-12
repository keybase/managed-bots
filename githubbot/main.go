package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"github.com/keybase/managed-bots/githubbot/githubbot"
	"golang.org/x/oauth2"
	oauth2github "golang.org/x/oauth2/github"
	"golang.org/x/sync/errgroup"
)

type Options struct {
	*base.Options
	HTTPPrefix        string
	WebhookSecret     string
	PrivateKeyPath    string
	AppName           string
	AppID             int64
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
		Server: base.NewServer("githubbot", opts.Announcement, opts.AWSOpts, opts.MultiDSN, kbchat.RunOptions{
			KeybaseLocation: opts.KeybaseLocation,
			HomeDir:         opts.Home,
		}),
		opts: opts,
	}
}

const backs = "```"

func (s *BotServer) makeAdvertisement() kbchat.Advertisement {
	subExtended := fmt.Sprintf(`Enables posting updates from the provided GitHub repository to this conversation.

Running this command without a branch or event type will subscribe you to all events on the specified repository's default branch.

Event type must be one of %sissues, pulls, commits, statuses%s

Examples:%s
!github subscribe keybase/client
!github subscribe microsoft/typescript pulls
!github subscribe facebook/react gh-pages%s`,
		backs, backs, backs, backs)

	unsubExtended := fmt.Sprintf(`Disables updates from the provided GitHub repository to this conversation.

Running this command without a branch or event type will unsubscribe you from all events on the specified repository.

Event type must be one of %sissues, pulls, commits, statuses%s

Examples:%s
!github unsubscribe keybase/client
!github unsubscribe microsoft/typescript commits
!github unsubscribe facebook/react gh-pages%s`,
		backs, backs, backs, backs)

	mentionsExtended := fmt.Sprintf(`Enables or disables mentions in GitHub events that involve your proven GitHub username.

Examples:%s
!github mentions disable
!github mentions enable%s
	`, backs, backs)

	cmds := []chat1.UserBotCommandInput{
		{
			Name:        "github subscribe",
			Description: "Enable updates from GitHub repos",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!github subscribe* <owner/repo> [branch or event type]`,
				DesktopBody: subExtended,
				MobileBody:  subExtended,
			},
		},
		{
			Name:        "github unsubscribe",
			Description: "Disable updates from GitHub repos",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!github unsubscribe* <owner/repo> [branch or event type]`,
				DesktopBody: unsubExtended,
				MobileBody:  unsubExtended,
			},
		},
		{
			Name:        "github mentions",
			Description: "Enable or disable mentions in GitHub events for your username in the current conversation.",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!github mentions* <disable/enable>`,
				DesktopBody: mentionsExtended,
				MobileBody:  mentionsExtended,
			},
		},
		{
			Name:        "github list",
			Description: "List subscriptions for the current conversation.",
		},
		base.GetFeedbackCommandAdvertisement(s.kbc.GetUsername()),
	}
	return kbchat.Advertisement{
		Alias: "GitHub",
		Advertisements: []chat1.AdvertiseCommandAPIParam{
			{
				Typ:      "public",
				Commands: cmds,
			},
		},
	}
}

func (s *BotServer) getAppKey() ([]byte, error) {
	if s.opts.PrivateKeyPath != "" {
		keyFile, err := os.Open(s.opts.PrivateKeyPath)
		if err != nil {
			return []byte{}, err
		}
		defer keyFile.Close()

		b, err := ioutil.ReadAll(keyFile)
		if err != nil {
			return []byte{}, err
		}

		return b, nil
	}

	path := fmt.Sprintf("/keybase/private/%s/bot.private-key.pem", s.kbc.GetUsername())
	cmd := s.opts.Command("fs", "read", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	s.Debug("Running `keybase fs read` on %q and waiting for it to finish...\n", path)
	if err := cmd.Run(); err != nil {
		return []byte{}, err
	}
	return out.Bytes(), nil
}

type botConfig struct {
	AppName       string `json:"app_name"`
	AppID         int64  `json:"app_id"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	WebhookSecret string `json:"webhook_secret"`
}

func (s *BotServer) getConfig() (config *botConfig, err error) {
	if s.opts.OAuthClientID != "" && s.opts.OAuthClientSecret != "" && s.opts.AppName != "" && s.opts.AppID != -1 {
		return &botConfig{
			s.opts.AppName,
			s.opts.AppID,
			s.opts.OAuthClientID,
			s.opts.OAuthClientSecret,
			s.opts.WebhookSecret,
		}, nil
	}
	path := fmt.Sprintf("/keybase/private/%s/credentials.json", s.kbc.GetUsername())
	cmd := s.opts.Command("fs", "read", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	s.Debug("Running `keybase fs read` on %q and waiting for it to finish...\n", path)
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(out.Bytes(), &config); err != nil {
		return nil, err
	}

	return config, nil
}

func (s *BotServer) Go() (err error) {
	if s.kbc, err = s.Start(s.opts.ErrReportConv); err != nil {
		return err
	}

	botConfig, err := s.getConfig()
	if err != nil {
		s.Errorf("failed to get bot configuration: %s", err)
		return
	}

	appKey, err := s.getAppKey()
	if err != nil {
		s.Errorf("failed to get private key: %s", err)
	}
	sdb, err := sql.Open("mysql", s.opts.DSN)
	if err != nil {
		s.Errorf("failed to connect to MySQL: %s", err)
		return err
	}
	defer sdb.Close()
	db := githubbot.NewDB(sdb)
	if _, err := s.kbc.AdvertiseCommands(s.makeAdvertisement()); err != nil {
		s.Errorf("advertise error: %s", err)
		return err
	}
	if err := s.SendAnnouncement(s.opts.Announcement, "I live."); err != nil {
		s.Errorf("failed to announce self: %s", err)
	}

	// If changing scopes, wipe tokens from DB
	config := &oauth2.Config{
		ClientID:     botConfig.ClientID,
		ClientSecret: botConfig.ClientSecret,
		Scopes:       []string{},
		Endpoint:     oauth2github.Endpoint,
		RedirectURL:  s.opts.HTTPPrefix + "/githubbot/oauth",
	}

	tr := http.DefaultTransport
	atr, err := ghinstallation.NewAppsTransport(tr, botConfig.AppID, appKey)
	if err != nil {
		s.Errorf("failed to make github apps transport: %s", err)
		return err
	}
	debugConfig := base.NewChatDebugOutputConfig(s.kbc, s.opts.ErrReportConv)
	stats, err := base.NewStatsRegistry(debugConfig, s.opts.StathatEZKey)
	if err != nil {
		s.Debug("unable to create stats", err)
		return err
	}
	stats = stats.SetPrefix(s.Name())
	handler := githubbot.NewHandler(stats, s.kbc, debugConfig, db, config, atr, s.opts.HTTPPrefix, botConfig.AppName)
	httpSrv := githubbot.NewHTTPSrv(stats, s.kbc, debugConfig, db, handler, config, atr, botConfig.WebhookSecret)
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
	fs.StringVar(&opts.OAuthClientID, "client-id", os.Getenv("BOT_OAUTH_CLIENT_ID"), "GitHub OAuth2 client ID")
	fs.StringVar(&opts.OAuthClientSecret, "client-secret", os.Getenv("BOT_OAUTH_CLIENT_SECRET"), "GitHub OAuth2 client secret")
	fs.StringVar(&opts.PrivateKeyPath, "private-key-path", "", "Path to GitHub app private key file")
	fs.StringVar(&opts.AppName, "app-name", "", "Github App name")
	fs.Int64Var(&opts.AppID, "app-id", -1, "GitHub App ID")
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
		fmt.Printf("error running chat loop: %v\n", err)
		return 3
	}
	return 0
}
