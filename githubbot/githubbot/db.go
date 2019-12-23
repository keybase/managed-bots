package githubbot

import (
	"database/sql"

	"github.com/keybase/managed-bots/base"
)

type DB struct {
	*base.DB
}

func NewDB(db *sql.DB) *DB {
	return &DB{
		DB: base.NewDB(db),
	}
}

func (d *DB) CreateSubscription(convID string, repo string, branch string) error {
	// TODO: ignore dupes with feedback?
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT IGNORE INTO subscriptions
			(conv_id, repo, branch)
			VALUES
			(?, ?, ?)
		`, base.ShortConvID(convID), repo, branch)
		return err
	})
}

func (d *DB) DeleteSubscription(convID string, repo string, branch string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM subscriptions
			WHERE (conv_id = ? AND repo = ? AND branch = ?)
		`, base.ShortConvID(convID), repo, branch)
		return err
	})
}

func (d *DB) DeleteSubscriptionsForRepo(convID string, repo string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM subscriptions
			WHERE (conv_id = ? AND repo = ?)
		`, base.ShortConvID(convID), repo)
		return err
	})
}

func (d *DB) GetSubscribedConvs(repo string, branch string) (res []string, err error) {
	rows, err := d.DB.Query(`
		SELECT conv_id
		FROM subscriptions
		WHERE (repo = ? AND branch = ?)
		GROUP BY conv_id
	`, repo, branch)
	if err != nil {
		return res, err
	}
	for rows.Next() {
		var convID string
		if err := rows.Scan(&convID); err != nil {
			return res, err
		}
		res = append(res, convID)
	}
	return res, nil
}

func (d *DB) GetSubscriptionExists(convID string, repo string, branch string) (exists bool, err error) {
	row := d.DB.QueryRow(`
	SELECT 1
	FROM subscriptions
	WHERE (conv_id = ? AND repo = ? AND branch = ?)
	GROUP BY conv_id
	`, base.ShortConvID(convID), repo, branch)
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

func (d *DB) GetSubscriptionForRepoExists(convID string, repo string) (exists bool, err error) {
	row := d.DB.QueryRow(`
	SELECT 1
	FROM subscriptions
	WHERE (conv_id = ? AND repo = ?)
	`, base.ShortConvID(convID), repo)
	var rowRes string
	err = row.Scan(&rowRes)
	switch err {
	case sql.ErrNoRows:
		return false, nil
	case nil:
		return true, nil
	default:
		return false, err
	}
}
