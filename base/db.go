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

type BaseOAuthDB struct {
	*DB
}

func NewBaseOAuthDB(db *sql.DB) *BaseOAuthDB {
	return &BaseOAuthDB{
		DB: NewDB(db),
	}
}

func (d *BaseOAuthDB) GetState(state string) (*OAuthRequest, error) {
	var oauthState OAuthRequest
	row := d.DB.QueryRow(`SELECT identifier, conv_id, msg_id, is_complete
		FROM oauth_state
		WHERE state = ?`, state)
	err := row.Scan(&oauthState.TokenIdentifier, &oauthState.ConvID,
		&oauthState.MsgID, &oauthState.IsComplete)
	switch err {
	case nil:
		return &oauthState, nil
	case sql.ErrNoRows:
		return nil, nil
	default:
		return nil, err
	}
}

func (d *BaseOAuthDB) PutState(state string, oauthState *OAuthRequest) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO oauth_state
		(state, identifier, conv_id, msg_id)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		identifier=VALUES(identifier),
		conv_id=VALUES(conv_id),
		msg_id=VALUES(msg_id)
	`, state, oauthState.TokenIdentifier, oauthState.ConvID, oauthState.MsgID)
		return err
	})
	return err
}

func (d *BaseOAuthDB) CompleteState(state string) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE oauth_state
		SET is_complete=true
		WHERE state = ?`, state)
		return err
	})
	return err
}

type OAuthDB struct {
	*BaseOAuthDB
}

func NewOAuthDB(db *sql.DB) *OAuthDB {
	return &OAuthDB{
		BaseOAuthDB: NewBaseOAuthDB(db),
	}
}

func (d *OAuthDB) GetToken(identifier string) (*oauth2.Token, error) {
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

func (d *OAuthDB) PutToken(identifier string, token *oauth2.Token) error {
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

func (d *OAuthDB) DeleteToken(identifier string) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM oauth
	WHERE identifier = ?`, identifier)
		return err
	})
	return err
}
