package meetbot

import (
	"database/sql"
	"time"

	"golang.org/x/oauth2"
)

type OAuthDB struct {
	db *sql.DB
}

func NewOAuthDB(db *sql.DB) *OAuthDB {
	return &OAuthDB{
		db: db,
	}
}

func (d *OAuthDB) GetToken(identifier string) (*oauth2.Token, error) {

	var token oauth2.Token
	var expiry int64
	row := d.db.QueryRow(`SELECT access_token, token_type, refresh_token, ROUND(UNIX_TIMESTAMP(expiry)*1000)
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

func (d *OAuthDB) PutToken(identifier string, token *oauth2.Token) error {
	_, err := d.db.Exec(`INSERT INTO oauth
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

func (d *OAuthDB) DeleteToken(identifier string) error {
	_, err := d.db.Exec(`DELETE FROM oauth
	WHERE identifier = ?`, identifier)
	return err
}
