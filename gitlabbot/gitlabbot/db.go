package gitlabbot

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"

	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type DB struct {
	*base.DB
}

type SubscribedConv struct {
	ConvID                chat1.ConvIDStr
	ReauthorizationNeeded bool
}

func NewDB(db *sql.DB) *DB {
	return &DB{
		DB: base.NewDB(db),
	}
}

// webhook subscription methods

func (d *DB) CreateSubscription(ctx context.Context, convID chat1.ConvIDStr, repo string, oauthIdentifier string) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO subscriptions
		(conv_id, repo, oauth_identifier)
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
		oauth_identifier=VALUES(oauth_identifier)
	`, convID, repo, oauthIdentifier)
	return err
}

func (d *DB) DeleteSubscription(ctx context.Context, convID chat1.ConvIDStr, repo string) error {
	_, err := d.ExecContext(ctx, `
		DELETE FROM subscriptions
		WHERE (conv_id = ? AND repo = ?)
	`, convID, repo)
	return err
}

func (d *DB) DeleteSubscriptionsForRepo(ctx context.Context, convID chat1.ConvIDStr, repo string) error {
	_, err := d.ExecContext(ctx, `
		DELETE FROM subscriptions
		WHERE (conv_id = ? AND repo = ?)
	`, convID, repo)
	return err
}

func (d *DB) GetSubscribedConvs(ctx context.Context, repo string) (res []SubscribedConv, err error) {
	rows, err := d.QueryContext(ctx, `
		SELECT conv_id, reauthorization_needed
		FROM subscriptions
		WHERE repo = ?
	`, repo)
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var subscribedConv SubscribedConv
		if err := rows.Scan(&subscribedConv.ConvID, &subscribedConv.ReauthorizationNeeded); err != nil {
			return res, err
		}
		res = append(res, subscribedConv)
	}
	return res, rows.Err()
}

func (d *DB) GetSubscriptionExists(ctx context.Context, convID chat1.ConvIDStr, repo string) (exists bool, err error) {
	row := d.QueryRowContext(ctx, `
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

func (d *DB) GetSubscriptionForRepoStatus(ctx context.Context, convID chat1.ConvIDStr, repo string) (
	exists bool, reauthorizationNeeded bool, err error,
) {
	row := d.QueryRowContext(ctx, `
	SELECT reauthorization_needed
	FROM subscriptions
	WHERE (conv_id = ? AND repo = ?)
	`, convID, repo)
	err = row.Scan(&reauthorizationNeeded)
	switch err {
	case sql.ErrNoRows:
		return false, false, nil
	case nil:
		return true, reauthorizationNeeded, nil
	default:
		return false, false, err
	}
}

func (d *DB) CompleteSubscriptionReauthorization(
	ctx context.Context, convID chat1.ConvIDStr, repo string,
) error {
	res, err := d.ExecContext(ctx, `
		UPDATE subscriptions
		SET reauthorization_needed = false
		WHERE conv_id = ? AND repo = ? AND reauthorization_needed = true
	`, convID, repo)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected != 1 {
		return fmt.Errorf("expected to complete one subscription reauthorization, updated %d", rowsAffected)
	}
	return nil
}

func (d *DB) GetAllSubscriptionsForConvID(ctx context.Context, convID chat1.ConvIDStr) (res []string, err error) {
	rows, err := d.QueryContext(ctx, `
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
	return res, rows.Err()
}

// OAuth2 token methods

func (d *DB) GetToken(ctx context.Context, identifier string) (*oauth2.Token, error) {
	var token oauth2.Token
	row := d.QueryRowContext(ctx, `SELECT access_token, token_type
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

func (d *DB) PutToken(ctx context.Context, identifier string, token *oauth2.Token) error {
	_, err := d.ExecContext(ctx, `INSERT INTO oauth
		(identifier, access_token, token_type, ctime, mtime)
		VALUES (?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
		access_token=VALUES(access_token),
		mtime=VALUES(mtime)
	`, identifier, token.AccessToken, token.TokenType)
	return err
}

func (d *DB) DeleteToken(ctx context.Context, identifier string) error {
	_, err := d.ExecContext(ctx, "DELETE FROM oauth WHERE identifier = ?", identifier)
	return err
}
