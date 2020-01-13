package base

import (
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/oauth2"
)

type DB struct {
	*sql.DB
}

func NewDB(db *sql.DB) *DB {
	return &DB{
		DB: db,
	}
}

func (d *DB) RunTxn(fn func(tx *sql.Tx) error) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			fmt.Printf("unable to rollback: %v", rerr)
		}
		return err
	}
	return tx.Commit()
}

type GoogleOAuthDB struct {
	*DB
}

func NewGoogleOAuthDB(db *sql.DB) *GoogleOAuthDB {
	return &GoogleOAuthDB{
		DB: NewDB(db),
	}
}

func (d *GoogleOAuthDB) GetToken(identifier string) (*oauth2.Token, error) {
	var token oauth2.Token
	var expiry int64
	row := d.DB.QueryRow(`SELECT access_token, token_type, refresh_token, ROUND(UNIX_TIMESTAMP(expiry))
		FROM oauth
		WHERE identifier = ?`, identifier)
	err := row.Scan(&token.AccessToken, &token.TokenType,
		&token.RefreshToken, &expiry)
	switch err {
	case nil:
		token.Expiry = time.Unix(expiry, 0)
		return &token, nil
	case sql.ErrNoRows:
		return nil, nil
	default:
		return nil, err
	}
}

func (d *GoogleOAuthDB) PutToken(identifier string, token *oauth2.Token) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO oauth
		(identifier, access_token, token_type, refresh_token, expiry, ctime, mtime)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
		access_token=VALUES(access_token),
		refresh_token=VALUES(refresh_token),
		expiry=VALUES(expiry),
		mtime=VALUES(mtime)
	`, identifier, token.AccessToken, token.TokenType, token.RefreshToken, token.Expiry)
		return err
	})
	return err
}

func (d *GoogleOAuthDB) DeleteToken(identifier string) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM oauth
	WHERE identifier = ?`, identifier)
		return err
	})
	return err
}
