package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/keybase/managed-bots/gitlabbot/gitlabbot"
	"os"

	_ "github.com/go-sql-driver/mysql"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
	oauth2gitlab "golang.org/x/oauth2/gitlab"
	"golang.org/x/sync/errgroup"
)

type Options struct {
	base.Options
	HTTPPrefix        string
	Secret            string
	OAuthClientID     string
	OAuthClientSecret string
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
	subExtended := fmt.Sprintf(`Enables posting updates from the provided GitLab repository to this conversation.

Example:%s
!gitlab subscribe keybase/client%s`,
		backs, backs)

	unsubExtended := fmt.Sprintf(`Disables updates from the provided GitLab repository to this conversation.

Example:%s
!gitlab unsubscribe keybase/client%s`,
		backs, backs)

	watchExtended := fmt.Sprintf(`Subscribes to updates from a non-default branch on the provided repo.
	
Example:%s
!gitlab watch facebook/react gh-pages%s`,
		backs, backs)

	unwatchExtended := fmt.Sprintf(`Disables updates from a non-default branch on the provided repo.

Example:%s
!gitlab unwatch facebook/react gh-pages%s
	`, backs, backs)

	mentionsExtended := fmt.Sprintf(`Enables or disables mentions in GitLab events that involve your proven GitLab username. 

Examples:%s
!gitlab mentions disable
!gitlab mentions enable%s
	`, backs, backs)

	cmds := []chat1.UserBotCommandInput{
		{
			Name:        "gitlab subscribe",
			Description: "Enable updates from GitLab repos",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!gitlab subscribe* <username/repo>`,
				DesktopBody: subExtended,
				MobileBody:  subExtended,
			},
		},
		{
			Name:        "gitlab watch",
			Description: "Watch pushes from branch",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!gitlab watch* <username/repo> <branch>`,
				DesktopBody: watchExtended,
				MobileBody:  watchExtended,
			},
		},
		{
			Name:        "gitlab unsubscribe",
			Description: "Disable updates from GitHub repos",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!gitlab unsubscribe* <username/repo>`,
				DesktopBody: unsubExtended,
				MobileBody:  unsubExtended,
			},
		},
		{
			Name:        "gitlab unwatch",
			Description: "Disable updates from branch",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!gitlab unwatch* <username/repo> <branch>`,
				DesktopBody: unwatchExtended,
				MobileBody:  unwatchExtended,
			},
		},
		{
			Name:        "gitlab mentions",
			Description: "Enable or disable mentions in GitHub events for your username.",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!gitlab mentions* <disable/enable>`,
				DesktopBody: mentionsExtended,
				MobileBody:  mentionsExtended,
			},
		},
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

func (s *BotServer) getSecret() (string, error) {
	if s.opts.Secret != "" {
		return s.opts.Secret, nil
	}
	path := fmt.Sprintf("/keybase/private/%s/bot.secret", s.kbc.GetUsername())
	cmd := s.opts.Command("fs", "read", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	s.Debug("Running `keybase fs read` on %q and waiting for it to finish...\n", path)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (s *BotServer) getOAuthConfig() (clientID string, clientSecret string, err error) {
	if s.opts.OAuthClientID != "" && s.opts.OAuthClientSecret != "" {
		return s.opts.OAuthClientID, s.opts.OAuthClientSecret, nil
	}
	path := fmt.Sprintf("/keybase/private/%s/credentials.json", s.kbc.GetUsername())
	cmd := s.opts.Command("fs", "read", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	s.Debug("Running `keybase fs read` on %q and waiting for it to finish...\n", path)
	if err := cmd.Run(); err != nil {
		return "", "", err
	}

	var j struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}

	if err := json.Unmarshal(out.Bytes(), &j); err != nil {
		return "", "", err
	}

	return j.ClientID, j.ClientSecret, nil
}

func (s *BotServer) Go() (err error) {
	if s.kbc, err = s.Start(s.opts.KeybaseLocation, s.opts.Home); err != nil {
		return err
	}

	secret, err := s.getSecret()
	if err != nil {
		s.Debug("failed to get secret: %s", err)
		return
	}

	clientID, clientSecret, err := s.getOAuthConfig()
	if err != nil {
		s.Debug("failed to get oauth credentials: %s", err)
		return
	}
	sdb, err := sql.Open("mysql", s.opts.DSN)
	if err != nil {
		s.Debug("failed to connect to MySQL: %s", err)
		return err
	}
	db := gitlabbot.NewDB(sdb)
	if _, err := s.kbc.AdvertiseCommands(s.makeAdvertisement()); err != nil {
		s.Debug("advertise error: %s", err)
		return err
	}
	if err := s.SendAnnouncement(s.opts.Announcement, "I live."); err != nil {
		s.Debug("failed to announce self: %s", err)
		return err
	}

	// If changing scopes, wipe tokens from DB
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"api","read_user"},
		Endpoint:     oauth2gitlab.Endpoint,
		RedirectURL:  s.opts.HTTPPrefix + "/gitlabbot/oauth",
	}

	requests := &base.OAuthRequests{}
	handler := gitlabbot.NewHandler(s.kbc, db, requests, config, s.opts.HTTPPrefix, secret)
	httpSrv := gitlabbot.NewHTTPSrv(s.kbc, db, handler, requests, config, secret)
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
	flag.StringVar(&opts.DSN, "dsn", os.Getenv("BOT_DSN"), "Bot database DSN")
	flag.StringVar(&opts.HTTPPrefix, "http-prefix", os.Getenv("BOT_HTTP_PREFIX"), "address of bots HTTP server for webhooks")
	flag.StringVar(&opts.Secret, "secret", os.Getenv("BOT_WEBHOOK_SECRET"), "Webhook secret")
	flag.StringVar(&opts.OAuthClientID, "client-id", os.Getenv("BOT_OAUTH_CLIENT_ID"), "GitLab OAuth2 client ID")
	flag.StringVar(&opts.OAuthClientSecret, "client-secret", os.Getenv("BOT_OAUTH_CLIENT_SECRET"), "GitLab OAuth2 client secret")
	flag.Parse()
	if len(opts.DSN) == 0 {
		fmt.Printf("must specify a poll database DSN\n")
		return 3
	}
	bs := NewBotServer(opts)
	if err := bs.Go(); err != nil {
		fmt.Printf("error running chat loop: %v\n", err)
	}
	return 0
}
