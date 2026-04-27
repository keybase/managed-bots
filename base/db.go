package base

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/oauth2"
)

const DefaultMaxOpenConns = 25

type DB struct {
	*sql.DB
}

func NewDB(db *sql.DB) *DB {
	db.SetMaxOpenConns(DefaultMaxOpenConns)
	return &DB{
		DB: db,
	}
}

func (d *DB) RunTxnContext(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		if rerr := tx.Rollback(); rerr != nil && !errors.Is(rerr, sql.ErrTxDone) {
			fmt.Printf("unable to rollback: %v", rerr)
		}
		return normalizeCtxErr(ctx, err)
	}
	return normalizeCommitErr(ctx, tx.Commit())
}

// normalizeCommitErr recovers the true cause of a failed Commit. When the
// context is cancelled, the driver rolls back the transaction and marks it
// done, so tx.Commit() returns sql.ErrTxDone instead of context.Canceled.
// Returning ctx.Err() lets callers distinguish a client abort from a real DB error.
func normalizeCommitErr(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, sql.ErrTxDone) {
		return ctxErr
	}
	return err
}

// normalizeCtxErr maps opaque driver errors (ErrBadConn, ErrInvalidConn) back
// to ctx.Err() when the context is already done. This lets callers distinguish
// a client abort from a real storage failure without checking at every call site.
func normalizeCtxErr(ctx context.Context, err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if ctxErr := ctx.Err(); ctxErr != nil &&
		(errors.Is(err, driver.ErrBadConn) || errors.Is(err, mysql.ErrInvalidConn)) {
		return ctxErr
	}
	return err
}

type BaseOAuthDB struct { //nolint
	*DB
}

func NewBaseOAuthDB(db *sql.DB) *BaseOAuthDB {
	return &BaseOAuthDB{
		DB: NewDB(db),
	}
}

func (d *BaseOAuthDB) GetState(ctx context.Context, state string) (*OAuthRequest, error) {
	var oauthState OAuthRequest
	row := d.QueryRowContext(ctx, `SELECT identifier, conv_id, msg_id, is_complete
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

func (d *BaseOAuthDB) PutState(ctx context.Context, state string, oauthState *OAuthRequest) error {
	_, err := d.ExecContext(ctx, `INSERT INTO oauth_state
		(state, identifier, conv_id, msg_id)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		identifier=VALUES(identifier),
		conv_id=VALUES(conv_id),
		msg_id=VALUES(msg_id)
	`, state, oauthState.TokenIdentifier, oauthState.ConvID, oauthState.MsgID)
	return err
}

func (d *BaseOAuthDB) CompleteState(ctx context.Context, state string) error {
	_, err := d.ExecContext(ctx, `UPDATE oauth_state
		SET is_complete=true
		WHERE state = ?`, state)
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

func (d *OAuthDB) GetToken(ctx context.Context, identifier string) (*oauth2.Token, error) {
	var token oauth2.Token
	var expiry int64
	row := d.QueryRowContext(ctx, `SELECT access_token, token_type, refresh_token, ROUND(UNIX_TIMESTAMP(expiry))
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

func (d *OAuthDB) PutToken(ctx context.Context, identifier string, token *oauth2.Token) error {
	_, err := d.ExecContext(ctx, `INSERT INTO oauth
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

func (d *OAuthDB) DeleteToken(ctx context.Context, identifier string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM oauth WHERE identifier = ?`, identifier)
	return err
}
