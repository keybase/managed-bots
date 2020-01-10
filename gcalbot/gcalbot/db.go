package gcalbot

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

func (d *DB) DeleteToken(identifier string) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := d.DB.Exec(`DELETE FROM oauth
	WHERE identifier = ?`, identifier)
		return err
	})
	return err
}

func (d *DB) GetAccountsForUser(username string) (accounts []interface{}, err error) {
	rows, err := d.DB.Query(`SELECT nickname
		FROM accounts
		WHERE username = ?
		ORDER BY nickname`, username)
	if err == sql.ErrNoRows {
		return accounts, nil
	} else if err != nil {
		return nil, err
	}
	for rows.Next() {
		var account string
		err = rows.Scan(&account)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func (d *DB) ExistsAccountForUser(username string, nickname string) (exists bool, err error) {
	row := d.DB.QueryRow(`SELECT EXISTS(
		SELECT * FROM accounts WHERE username = ? AND nickname = ?)`,
		username, nickname)
	err = row.Scan(&exists)
	return exists, err
}

func (d *DB) InsertAccountForUser(username string, nickname string) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO accounts
		(username, nickname)
		VALUES (?, ?)
	`, username, nickname)
		return err
	})
	return err
}

func (d *DB) DeleteAccountForUser(username string, nickname string) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := d.DB.Exec(`DELETE FROM accounts
	WHERE username = ? and nickname = ?`, username, nickname)
		return err
	})
	return err
}
