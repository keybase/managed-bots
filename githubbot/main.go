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
	Secret            string
	PrivateKeyPath    string
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
		Server: base.NewServer(opts.Announcement, opts.AWSOpts),
		opts:   opts,
	}
}

const backs = "```"

func (s *BotServer) makeAdvertisement() kbchat.Advertisement {
	subExtended := fmt.Sprintf(`Enables posting updates from the provided GitHub repository to this conversation.

Example:%s
!github subscribe keybase/client%s

Subscribe to a specific branch:%s
!github subscribe facebook/react gh-pages%s`,
		backs, backs, backs, backs)

	unsubExtended := fmt.Sprintf(`Disables updates from the provided GitHub repository to this conversation.

Example:%s
!github unsubscribe keybase/client%s

Unsubscribe from a specific branch:%s
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
				Title:       `*!github subscribe* <username/repo> [branch]`,
				DesktopBody: subExtended,
				MobileBody:  subExtended,
			},
		},
		{
			Name:        "github unsubscribe",
			Description: "Disable updates from GitHub repos",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!github unsubscribe* <username/repo> [branch]`,
				DesktopBody: unsubExtended,
				MobileBody:  unsubExtended,
			},
		},
		{
			Name:        "github mentions",
			Description: "Enable or disable mentions in GitHub events for your username.",
			ExtendedDescription: &chat1.UserBotExtendedDescription{
				Title:       `*!github mentions* <disable/enable>`,
				DesktopBody: mentionsExtended,
				MobileBody:  mentionsExtended,
			},
		},
		{
			Name:        "github auth",
			Description: "Check if GitHub is authenticated for your account.",
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

func (s *BotServer) getConfig() (appID int64, clientID string, clientSecret string, err error) {
	if s.opts.OAuthClientID != "" && s.opts.OAuthClientSecret != "" && s.opts.AppID != -1 {
		return s.opts.AppID, s.opts.OAuthClientID, s.opts.OAuthClientSecret, nil
	}
	path := fmt.Sprintf("/keybase/private/%s/credentials.json", s.kbc.GetUsername())
	cmd := s.opts.Command("fs", "read", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	s.Debug("Running `keybase fs read` on %q and waiting for it to finish...\n", path)
	if err := cmd.Run(); err != nil {
		return -1, "", "", err
	}

	var j struct {
		AppID        int64  `json:"app_id"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}

	if err := json.Unmarshal(out.Bytes(), &j); err != nil {
		return -1, "", "", err
	}

	return j.AppID, j.ClientID, j.ClientSecret, nil
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

	appID, clientID, clientSecret, err := s.getConfig()
	if err != nil {
		s.Debug("failed to get oauth credentials: %s", err)
		return
	}

	appKey, err := s.getAppKey()
	if err != nil {
		s.Debug("failed to get private key: %s", err)
	}
	sdb, err := sql.Open("mysql", s.opts.DSN)
	if err != nil {
		s.Debug("failed to connect to MySQL: %s", err)
		return err
	}
	defer sdb.Close()
	db := githubbot.NewDB(sdb)
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
		Scopes:       []string{},
		Endpoint:     oauth2github.Endpoint,
		RedirectURL:  s.opts.HTTPPrefix + "/githubbot/oauth",
	}

	requests := &base.OAuthRequests{}

	tr := http.DefaultTransport
	atr, err := ghinstallation.NewAppsTransport(tr, appID, appKey)
	if err != nil {
		s.Debug("failed to make github apps transport: %s", err)
		return err
	}
	handler := githubbot.NewHandler(s.kbc, db, requests, config, atr, s.opts.HTTPPrefix, secret)
	httpSrv := githubbot.NewHTTPSrv(s.kbc, db, handler, requests, config, atr, secret)
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
	fs.StringVar(&opts.OAuthClientID, "client-id", os.Getenv("BOT_OAUTH_CLIENT_ID"), "GitHub OAuth2 client ID")
	fs.StringVar(&opts.OAuthClientSecret, "client-secret", os.Getenv("BOT_OAUTH_CLIENT_SECRET"), "GitHub OAuth2 client secret")
	fs.StringVar(&opts.PrivateKeyPath, "private-key-path", "", "Path to GitHub app private key file")
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
