// Migration script to backfill the branches table with default branch rows
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/bradleyfalzon/ghinstallation"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/go-github/v31/github"

	"github.com/keybase/managed-bots/githubbot/githubbot"
)

func main() {
	rc := mainInner()
	os.Exit(rc)
}

func getAppKey(privateKeyPath string) ([]byte, error) {
	keyFile, err := os.Open(privateKeyPath)
	if err != nil {
		return []byte{}, err
	}
	defer keyFile.Close()

	b, err := io.ReadAll(keyFile)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}

func mainInner() int {
	var privateKeyPath string
	var appID int64
	var dsn string
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.StringVar(&privateKeyPath, "private-key-path", "", "Path to GitHub app private key file")
	fs.Int64Var(&appID, "app-id", -1, "GitHub App ID")
	fs.StringVar(&dsn, "dsn", os.Getenv("BOT_DSN"), "Bot database DSN")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Printf("failed to parse options: %s", err)
		return 1
	}

	if len(dsn) == 0 {
		fmt.Printf("must specify a database DSN\n")
		return 3
	}

	appKey, err := getAppKey(privateKeyPath)
	if err != nil {
		fmt.Printf("failed to get private key: %s", err)
	}
	sdb, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Printf("failed to connect to MySQL: %s", err)
		return 1
	}
	defer sdb.Close()
	db := githubbot.NewDB(sdb)

	tr := http.DefaultTransport
	atr, err := ghinstallation.NewAppsTransport(tr, appID, appKey)
	if err != nil {
		fmt.Printf("failed to make github apps transport: %s", err)
		return 1
	}

	subs, err := db.GetAllSubscriptions()
	if err != nil {
		fmt.Printf("failed to get all subscriptions: %s", err)
		return 1
	}

	fmt.Printf("Found %d subscriptions to migrate\n", len(subs))
	for i, subscription := range subs {
		itr := ghinstallation.NewFromAppsTransport(atr, subscription.InstallationID)
		client := github.NewClient(&http.Client{Transport: itr})

		defaultBranch, err := githubbot.GetDefaultBranch(subscription.Repo, client)
		if err != nil {
			fmt.Printf("Error getting default branch for subscription %d/%d: %s\n", i, len(subs), err)
			continue
		}

		err = db.WatchBranch(subscription.ConvID, subscription.Repo, defaultBranch)
		if err != nil {
			fmt.Printf("Error watching branch: %s", err)
			return 1
		}
	}

	return 0
}
