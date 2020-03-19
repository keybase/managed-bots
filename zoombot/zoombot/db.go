package zoombot

import (
	"database/sql"

	"github.com/keybase/managed-bots/base"
)

type DB struct {
	*base.OAuthDB
}

func NewDB(db *sql.DB) *DB {
	return &DB{
		OAuthDB: base.NewOAuthDB(db),
	}
}

func (d *DB) CreateUser(userID, accountID, identifier string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO user
			(user_id, account_id, identifier)
			VALUES (?, ?, ?)
		`, userID, accountID, identifier)
		return err
	})
}

func (d *DB) DeleteUserAndToken(userID, accountID string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE user, oauth
			FROM user
			JOIN oauth USING(identifier)
			WHERE user_id = ? AND account_id = ?
		`, userID, accountID)
		return err
	})
}
