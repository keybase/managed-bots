package base

import (
	"context"
	"database/sql"

	"golang.org/x/oauth2"
)

// OAuthStateDB implements the confirmation steps of the OAuth flow on an
// oauth_state table with state, requester, is_complete, ctime, pending_token,
// confirm_code_hash and confirm_attempts columns. Each bot adds GetState and
// PutState for its own columns.
type OAuthStateDB struct {
	*DB
}

func NewOAuthStateDB(db *DB) *OAuthStateDB {
	return &OAuthStateDB{DB: db}
}

// SetPendingToken stores the token from the provider until the requester
// confirms it. It returns false if the state was already used or expired.
func (d *OAuthStateDB) SetPendingToken(ctx context.Context, state, codeHash, pendingToken string) (bool, error) {
	res, err := d.ExecContext(ctx, `UPDATE oauth_state
		SET pending_token=?, confirm_code_hash=?, confirm_attempts=0
		WHERE state = ? AND is_complete=false AND pending_token IS NULL
		AND ctime > NOW() - INTERVAL ? SECOND`, pendingToken, codeHash, state, OAuthStateTTLSeconds)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// ReissueConfirmCode replaces the confirmation code of a pending state, for a
// reload of the callback page in the browser that authorized. The attempt count
// is kept. It returns false if the state isn't pending.
func (d *OAuthStateDB) ReissueConfirmCode(ctx context.Context, state, codeHash string) (bool, error) {
	res, err := d.ExecContext(ctx, `UPDATE oauth_state
		SET confirm_code_hash=?
		WHERE state = ? AND is_complete=false AND pending_token IS NOT NULL
		AND ctime > NOW() - INTERVAL ? SECOND`, codeHash, state, OAuthStateTTLSeconds)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (d *OAuthStateDB) GetPendingCodes(ctx context.Context, requester string) (res []PendingOAuthCode, err error) {
	rows, err := d.QueryContext(ctx, `SELECT state, confirm_code_hash
		FROM oauth_state
		WHERE requester = ? AND is_complete=false AND pending_token IS NOT NULL
		AND confirm_attempts < ? AND ctime > NOW() - INTERVAL ? SECOND`,
		requester, MaxConfirmAttempts, OAuthStateTTLSeconds)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); err == nil {
			err = cerr
		}
	}()
	for rows.Next() {
		var p PendingOAuthCode
		if err := rows.Scan(&p.State, &p.CodeHash); err != nil {
			return nil, err
		}
		res = append(res, p)
	}
	return res, rows.Err()
}

// GetPendingToken returns nil if state isn't pending.
func (d *OAuthStateDB) GetPendingToken(ctx context.Context, state string) (*oauth2.Token, error) {
	var pendingToken string
	row := d.QueryRowContext(ctx, `SELECT pending_token
		FROM oauth_state
		WHERE state = ? AND is_complete=false AND pending_token IS NOT NULL
		AND ctime > NOW() - INTERVAL ? SECOND`, state, OAuthStateTTLSeconds)
	switch err := row.Scan(&pendingToken); err {
	case nil:
		return UnmarshalPendingToken(pendingToken)
	case sql.ErrNoRows:
		return nil, nil
	default:
		return nil, err
	}
}

// IncrementConfirmAttempts counts a wrong code against every pending state of
// the requester.
func (d *OAuthStateDB) IncrementConfirmAttempts(ctx context.Context, requester string) error {
	_, err := d.ExecContext(ctx, `UPDATE oauth_state
		SET confirm_attempts=confirm_attempts+1
		WHERE requester = ? AND is_complete=false AND pending_token IS NOT NULL`, requester)
	return err
}

// CompleteState marks state used and drops its pending token. It returns
// false if the state was already complete, so a link is confirmed only once.
func (d *OAuthStateDB) CompleteState(ctx context.Context, state string) (bool, error) {
	res, err := d.ExecContext(ctx, `UPDATE oauth_state
		SET is_complete=true, pending_token=NULL, confirm_code_hash=NULL
		WHERE state = ? AND is_complete=false`, state)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
