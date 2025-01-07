package gitlabbot

import (
	"database/sql"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"

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

func (d *DB) CreateSubscription(convID chat1.ConvIDStr, repo string, oauthIdentifier string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO subscriptions
			(conv_id, repo, oauth_identifier)
			VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE
			oauth_identifier=VALUES(oauth_identifier)
		`, convID, repo, oauthIdentifier)
		return err
	})
}

func (d *DB) DeleteSubscription(convID chat1.ConvIDStr, repo string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM subscriptions
			WHERE (conv_id = ? AND repo = ?)
		`, convID, repo)
		return err
	})
}

func (d *DB) DeleteSubscriptionsForRepo(convID chat1.ConvIDStr, repo string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM subscriptions
			WHERE (conv_id = ? AND repo = ?)
		`, convID, repo)
		return err
	})
}

func (d *DB) GetSubscribedConvs(repo string) (res []chat1.ConvIDStr, err error) {
	rows, err := d.DB.Query(`
		SELECT conv_id
		FROM subscriptions
		WHERE repo = ?
		GROUP BY conv_id
	`, repo)
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

func (d *DB) GetSubscriptionExists(convID chat1.ConvIDStr, repo string) (exists bool, err error) {
	row := d.DB.QueryRow(`
	SELECT 1
	FROM subscriptions
	WHERE (conv_id = ? AND repo = ?)
	GROUP BY conv_id
	`, convID, repo)
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
	WHERE (conv_id = ? AND repo = ?)
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

func (d *DB) GetAllSubscriptionsForConvID(convID chat1.ConvIDStr) (res []string, err error) {
	rows, err := d.DB.Query(`
		SELECT repo
		FROM subscriptions
		WHERE conv_id = ?
		ORDER BY repo
	`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var repo string
		if err := rows.Scan(&repo); err != nil {
			return res, err
		}
		res = append(res, repo)
	}
	return res, nil
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
		_, err := tx.Exec("DELETE FROM oauth WHERE identifier = ?", identifier)
		return err
	})
	return err
}
