package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/keybase/managed-bots/gitlabbot/gitlabbot"

	_ "github.com/go-sql-driver/mysql"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
	oauth2gitlab "golang.org/x/oauth2/gitlab"
	"golang.org/x/sync/errgroup"
)

type Options struct {
	*base.Options
	HTTPPrefix        string
	Secret            string
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
		Server: base.NewServer(opts.Announcement, opts.AWSOpts),
		opts:   opts,
	}
}

const backs = "```"

func (s *BotServer) makeAdvertisement() kbchat.Advertisement {
	subExtended := fmt.Sprintf(`Enables posting updates from the provided GitLab project to this conversation.

Example:%s
!gitlab subscribe keybase/client%s

Subscribe to a specific branch:%s
!gitlab subscribe facebook/react gh-pages%s`,
		backs, backs, backs, backs)

	unsubExtended := fmt.Sprintf(`Disables updates from the provided GitLab project to this conversation.

Example:%s
!gitlab unsubscribe keybase/client%s

Unsubscribe from a specific branch:%s
!gitlab unsubscribe facebook/react gh-pages%s`,
		backs, backs, backs, backs)

	cmds := []chat1.UserBotCommandInput{
		{
			Name:        "gitlab subscribe",
			Description: "Enable updates from GitLab projects",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!gitlab subscribe* <username/project> [branch]`,
				DesktopBody: subExtended,
				MobileBody:  subExtended,
			},
		},
		{
			Name:        "gitlab unsubscribe",
			Description: "Disable updates from GitLab projects",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!gitlab unsubscribe* <username/project> [branch]`,
				DesktopBody: unsubExtended,
				MobileBody:  unsubExtended,
			},
		},
	}
	return kbchat.Advertisement{
		Alias: "GitLab",
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
	defer sdb.Close()
	db := gitlabbot.NewDB(sdb)
	if _, err := s.kbc.AdvertiseCommands(s.makeAdvertisement()); err != nil {
		s.Debug("advertise error: %s", err)
		return err
	}
	if err := s.SendAnnouncement(s.opts.Announcement, "I live."); err != nil {
		s.Debug("failed to announce self: %s", err)
	}

	// If changing scopes, wipe tokens from DB
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"api"},
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
	opts := NewOptions()
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.StringVar(&opts.HTTPPrefix, "http-prefix", os.Getenv("BOT_HTTP_PREFIX"), "address of bots HTTP server for webhooks")
	fs.StringVar(&opts.Secret, "secret", os.Getenv("BOT_WEBHOOK_SECRET"), "Webhook secret")
	fs.StringVar(&opts.OAuthClientID, "client-id", os.Getenv("BOT_OAUTH_CLIENT_ID"), "GitLab OAuth2 client ID")
	fs.StringVar(&opts.OAuthClientSecret, "client-secret", os.Getenv("BOT_OAUTH_CLIENT_SECRET"), "GitLab OAuth2 client secret")
	if err := opts.Parse(fs, os.Args); err != nil {
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
