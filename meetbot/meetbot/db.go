package meetbot

import (
	"database/sql"
	"time"

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

func (d *DB) GetToken(identifier string) (*oauth2.Token, error) {

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

func (d *DB) PutToken(identifier string, token *oauth2.Token) error {
	_, err := d.DB.Exec(`INSERT INTO oauth
		(identifier, access_token, token_type, refresh_token, expiry, ctime, mtime)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
		access_token=VALUES(access_token),
		refresh_token=VALUES(refresh_token),
		expiry=VALUES(expiry),
		mtime=VALUES(mtime)
	`, identifier, token.AccessToken, token.TokenType, token.RefreshToken, token.Expiry)
	return err
}

func (d *DB) DeleteToken(identifier string) error {
	_, err := d.DB.Exec(`DELETE FROM oauth
	WHERE identifier = ?`, identifier)
	return err
}
