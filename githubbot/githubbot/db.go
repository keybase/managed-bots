package githubbot

import (
	"database/sql"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type DB struct {
	*base.BaseOAuthDB
}

func NewDB(db *sql.DB) *DB {
	return &DB{
		BaseOAuthDB: base.NewBaseOAuthDB(db),
	}
}

// webhook subscription methods

func (d *DB) CreateSubscription(convID chat1.ConvIDStr, repo string, installationID int64) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO subscriptions
			(conv_id, repo, installation_id)
			VALUES
			(?, ?, ?)
			ON DUPLICATE KEY UPDATE
			installation_id=VALUES(installation_id)
		`, convID, repo, installationID)
		return err
	})
}

func (d *DB) DeleteSubscription(convID chat1.ConvIDStr, repo string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM subscriptions
			WHERE conv_id = ? AND repo = ?
		`, convID, repo)
		return err
	})
}

func (d *DB) WatchBranch(convID chat1.ConvIDStr, repo string, branch string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT IGNORE INTO branches
			(conv_id, repo, branch)
			VALUES
			(?, ?, ?)
		`, convID, repo, branch)
		return err
	})
}

func (d *DB) UnwatchBranch(convID chat1.ConvIDStr, repo string, branch string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM branches
			WHERE conv_id = ? AND repo = ? AND branch = ?
		`, convID, repo, branch)
		return err
	})
}

func (d *DB) DeleteSubscriptionsForRepo(convID chat1.ConvIDStr, repo string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM subscriptions
			WHERE conv_id = ? AND repo = ?
		`, convID, repo)
		return err
	})
}

func (d *DB) DeleteBranchesForRepo(convID chat1.ConvIDStr, repo string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM subscriptions
			WHERE conv_id = ? AND repo = ?
		`, convID, repo)
		return err
	})
}

func (d *DB) GetConvIDsFromRepoInstallation(repo string, installationID int64) (res []chat1.ConvIDStr, err error) {
	rows, err := d.DB.Query(`
		SELECT conv_id
		FROM subscriptions
		WHERE repo = ? AND installation_id = ?
		GROUP BY conv_id
	`, repo, installationID)
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var convID chat1.ConvIDStr
		if err := rows.Scan(&convID); err != nil {
			return res, err
		}
		res = append(res, convID)
	}
	return res, nil
}

func (d *DB) GetSubscriptionForBranchExists(convID chat1.ConvIDStr, repo string, branch string) (exists bool, err error) {
	row := d.DB.QueryRow(`
	SELECT 1
	FROM branches
	WHERE conv_id = ? AND repo = ? AND branch = ?
	GROUP BY conv_id
	`, convID, repo, branch)
	var rowRes string
	scanErr := row.Scan(&rowRes)
	switch scanErr {
	case sql.ErrNoRows:
		return false, nil
	case nil:
		return true, nil
	default:
		return false, scanErr
	}
}

func (d *DB) GetSubscriptionForRepoExists(convID chat1.ConvIDStr, repo string) (exists bool, err error) {
	row := d.DB.QueryRow(`
	SELECT 1
	FROM subscriptions
	WHERE conv_id = ? AND repo = ?
	`, convID, repo)
	var rowRes string
	err = row.Scan(&rowRes)
	switch err {
	case sql.ErrNoRows:
		return false, nil
	case nil:
		return true, nil
	default:
		return false, err
	}
}

func (d *DB) GetAllBranchesForRepo(convID chat1.ConvIDStr, repo string) ([]string, error) {
	rows, err := d.DB.Query(`SELECT branch
		FROM branches
		WHERE conv_id = ? AND repo = ?`, convID, repo)
	if err != nil {
		return nil, err
	}
	res := []string{}
	defer rows.Close()
	for rows.Next() {
		var branch string
		if err := rows.Scan(&branch); err != nil {
			return res, err
		}
		res = append(res, branch)
	}
	return res, nil
}

// subscription preferences

type Features struct {
	Issues       bool
	PullRequests bool
	Commits      bool
	Statuses     bool
	Releases     bool
}

func (f *Features) String() string {
	if f == nil {
		return "all events"
	}
	var res []string
	if f.Issues {
		res = append(res, "issues")
	}
	if f.PullRequests {
		res = append(res, "pull requests")
	}
	if f.Commits {
		res = append(res, "commits")
	}
	if f.Statuses {
		res = append(res, "commit statuses")
	}
	if f.Releases {
		res = append(res, "releases")
	}
	if len(res) == 0 {
		return "no events"
	} else if len(res) == 5 {
		return "all events"
	}
	return strings.Join(res, ", ")
}

func (d *DB) SetFeatures(convID chat1.ConvIDStr, repo string, features *Features) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO features
			(conv_id, repo, issues, pull_requests, commits, statuses, releases)
			VALUES
			(?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
			issues=VALUES(issues),
			pull_requests=VALUES(pull_requests),
			commits=VALUES(commits),
			statuses=VALUES(statuses)
			releases=VALUES(releases)
		`, convID, repo, features.Issues, features.PullRequests, features.Commits, features.Statuses, features.Releases)
		return err
	})
}

func (d *DB) GetFeatures(convID chat1.ConvIDStr, repo string) (*Features, error) {
	row := d.DB.QueryRow(`SELECT issues, pull_requests, commits, statuses, releases
		FROM features
		WHERE conv_id = ? AND repo = ?`, convID, repo)
	features := &Features{}
	err := row.Scan(&features.Issues, &features.PullRequests, &features.Commits, &features.Statuses, &features.Releases)
	switch err {
	case nil:
		return features, nil
	case sql.ErrNoRows:
		return nil, nil
	default:
		return nil, err
	}
}

func (d *DB) GetFeaturesForAllRepos(convID chat1.ConvIDStr) (map[string]Features, error) {
	rows, err := d.DB.Query(`SELECT repo, COALESCE(issues, true), COALESCE(pull_requests, true),
		COALESCE(commits, true), COALESCE(statuses, true), COALESCE(releases, true)
		FROM subscriptions
		LEFT JOIN features USING(conv_id, repo)
		WHERE conv_id = ?`, convID)
	if err != nil {
		return nil, err
	}
	res := make(map[string]Features)
	defer rows.Close()
	for rows.Next() {
		var repo string
		var features Features
		if err := rows.Scan(&repo, &features.Issues, &features.PullRequests, &features.Commits, &features.Statuses, &features.Releases); err != nil {
			return nil, err
		}
		res[repo] = features
	}
	return res, nil
}

func (d *DB) DeleteFeaturesForRepo(convID chat1.ConvIDStr, repo string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM features
			WHERE conv_id = ? AND repo = ?
		`, convID, repo)
		return err
	})
}

// OAuth2 token methods

func (d *DB) GetToken(identifier string) (*oauth2.Token, error) {
	var token oauth2.Token
	row := d.DB.QueryRow(`SELECT access_token, token_type
		FROM oauth
		WHERE identifier = ?`, identifier)
	err := row.Scan(&token.AccessToken, &token.TokenType)
	switch err {
	case nil:
		return &token, nil
	case sql.ErrNoRows:
		return nil, nil
	default:
		return nil, err
	}
}

func (d *DB) PutToken(identifier string, token *oauth2.Token) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO oauth
		(identifier, access_token, token_type, ctime, mtime)
		VALUES (?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
		access_token=VALUES(access_token),
		mtime=VALUES(mtime)
	`, identifier, token.AccessToken, token.TokenType)
		return err
	})
	return err
}

func (d *DB) DeleteToken(identifier string) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := d.DB.Exec(`DELETE FROM oauth
	WHERE identifier = ?`, identifier)
		return err
	})
	return err
}

// preferences

type UserPreferences struct {
	Mention bool
}

func (d *DB) GetUserPreferences(username string, convID chat1.ConvIDStr) (*UserPreferences, error) {
	row := d.DB.QueryRow(`SELECT mention
		FROM user_prefs
		WHERE username = ? AND conv_id = ?`, username, convID)
	prefs := &UserPreferences{}
	err := row.Scan(&prefs.Mention)
	switch err {
	case nil:
		return prefs, nil
	case sql.ErrNoRows:
		// if we don't have preferences saved for a user, return default preferences
		return &UserPreferences{
			Mention: true,
		}, nil
	default:
		return nil, err
	}
}

func (d *DB) SetUserPreferences(username string, convID chat1.ConvIDStr, prefs *UserPreferences) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO user_prefs
		(username, conv_id, mention)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
		mention=VALUES(mention)
	`, username, convID, prefs.Mention)
		return err
	})
	return err
}

// util
type DBSubscription struct {
	ConvID         chat1.ConvIDStr
	Repo           string
	InstallationID int64
}

func (d *DB) GetAllSubscriptions() (res []DBSubscription, err error) {
	rows, err := d.DB.Query(`
	SELECT conv_id, repo, installation_id
	FROM subscriptions
`)
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var subscription DBSubscription
		if err := rows.Scan(&subscription.ConvID, &subscription.Repo, &subscription.InstallationID); err != nil {
			return res, err
		}
		res = append(res, subscription)
	}
	return res, nil
}
