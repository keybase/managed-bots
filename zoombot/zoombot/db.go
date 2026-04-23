package zoombot

import (
	"context"
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

func (d *DB) CreateUser(ctx context.Context, userID, accountID, identifier string) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO user
		(user_id, account_id, identifier)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
		identifier=identifier -- identifier stays the same
	`, userID, accountID, identifier)
	return err
}

func (d *DB) DeleteUserAndToken(ctx context.Context, userID, accountID string) error {
	_, err := d.ExecContext(ctx, `
		DELETE user, oauth
		FROM user
		JOIN oauth USING(identifier)
		WHERE user_id = ? AND account_id = ?
	`, userID, accountID)
	return err
}
