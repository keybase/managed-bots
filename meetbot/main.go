package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
	"github.com/keybase/managed-bots/meetbot/meetbot"
	"github.com/op/go-logging"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

var log = logging.MustGetLogger("meetbot")

func main() {
	rc := mainInner()
	os.Exit(rc)
}

func mainInner() int {
	var opts meetbot.Options
	flag.StringVar(&opts.KeybaseLocation, "keybase", "keybase", "keybase command")
	flag.StringVar(&opts.Home, "home", "", "Home directory")
	flag.StringVar(&opts.Announcement, "announcement", os.Getenv("BOT_ANNOUNCEMENT"),
		"Where to announce we are running")

	var dsn string
	flag.StringVar(&dsn, "dsn", os.Getenv("BOT_DSN"), "bot database DSN")

	var kbfsRoot string
	flag.StringVar(&kbfsRoot, "kbfs-root", os.Getenv("BOT_KBFS_ROOT"), "root path to bot's KBFS backed config")

	flag.StringVar(&opts.HTTPAddr, "http-addr", os.Getenv("BOT_HTTP_ADDR"), "address of bots HTTP server for oauth requests")

	flag.Parse()
	if len(dsn) == 0 {
		fmt.Printf("must specify a BOT_DSN for bot database\n")
		return 3
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Printf("failed to connect to MySQL: %s\n", err)
		return 3
	}

	if len(kbfsRoot) == 0 {
		fmt.Printf("BOT_KBFS_ROOT must be specified\n")
		return 3
	}
	configPath := filepath.Join(kbfsRoot, "credentials.json")
	cmd := exec.Command("keybase", "fs", "read", configPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	fmt.Printf("Running `keybase fs read` on %q and waiting for it to finish...\n", configPath)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Command finished with error: %v\n", err)
		return 3
	}

	if len(opts.HTTPAddr) == 0 {
		opts.HTTPAddr = ":8080"
	}

	// If modifying these scopes, drop the saved tokens in the db
	config, err := google.ConfigFromJSON(out.Bytes(), calendar.CalendarEventsScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	srv := meetbot.NewBotServer(opts, config, meetbot.NewOAuthDB(db))
	if err := srv.Start(); err != nil {
		log.Fatalf("error running chat loop: %v\n", err)
		return 3
	}
	return 0
}
