package githubbot

import (
	"database/sql"

	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type DB struct {
	*base.DB
}

func NewDB(db *sql.DB) *DB {
	return &DB{
		DB: base.NewDB(db),
	}
}

// webhook subscription methods

func (d *DB) CreateSubscription(shortConvID base.ShortID, repo string, branch string, hookID int64) error {
	// TODO: ignore dupes with feedback?
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT IGNORE INTO subscriptions
			(conv_id, repo, branch, hook_id)
			VALUES
			(?, ?, ?, ?)
		`, shortConvID, repo, branch, hookID)
		return err
	})
}

func (d *DB) DeleteSubscription(shortConvID base.ShortID, repo string, branch string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM subscriptions
			WHERE (conv_id = ? AND repo = ? AND branch = ?)
		`, shortConvID, repo, branch)
		return err
	})
}

func (d *DB) DeleteSubscriptionsForRepo(shortConvID base.ShortID, repo string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM subscriptions
			WHERE (conv_id = ? AND repo = ?)
		`, shortConvID, repo)
		return err
	})
}

func (d *DB) GetSubscribedConvs(repo string, branch string) (res []base.ShortID, err error) {
	rows, err := d.DB.Query(`
		SELECT conv_id
		FROM subscriptions
		WHERE (repo = ? AND branch = ?)
		GROUP BY conv_id
	`, repo, branch)
	if err != nil {
		return res, err
	}
	for rows.Next() {
		var convID string
		if err := rows.Scan(&convID); err != nil {
			return res, err
		}
		res = append(res, base.ShortID(convID))
	}
	return res, nil
}

func (d *DB) GetSubscriptionExists(shortConvID base.ShortID, repo string, branch string) (exists bool, err error) {
	row := d.DB.QueryRow(`
	SELECT 1
	FROM subscriptions
	WHERE (conv_id = ? AND repo = ? AND branch = ?)
	GROUP BY conv_id
	`, shortConvID, repo, branch)
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

func (d *DB) GetSubscriptionForRepoExists(shortConvID base.ShortID, repo string) (exists bool, err error) {
	row := d.DB.QueryRow(`
	SELECT 1
	FROM subscriptions
	WHERE (conv_id = ? AND repo = ?)
	`, shortConvID, repo)
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

func (d *DB) GetHookIDForRepo(shortConvID base.ShortID, repo string) (hookID int64, err error) {
	row := d.DB.QueryRow(`
	SELECT hook_id
	FROM subscriptions
	WHERE (conv_id = ? AND repo = ?)
	`, shortConvID, repo)
	err = row.Scan(&hookID)
	if err != nil {
		return -1, err
	}

	return hookID, nil
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
