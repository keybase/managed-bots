package githubbot

import (
	"database/sql"
)

type DB struct {
	db *sql.DB
}

func NewDB(db *sql.DB) *DB {
	return &DB{
		db: db,
	}
}

func (d *DB) runTxn(fn func(tx *sql.Tx) error) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (d *DB) CreateSubscription(convID string, repo string, branch string) error {
	// TODO: ignore dupes with feedback?
	return d.runTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT IGNORE INTO subscriptions
			(conv_id, repo, branch)
			VALUES
			(?, ?, ?)
		`, shortConvID(convID), repo, branch)
		return err
	})
}

func (d *DB) DeleteOneSubscription(convID string, repo string, branch string) error {
	return d.runTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM subscriptions
			WHERE (conv_id = ? AND repo = ? AND branch = ?)
		`, shortConvID(convID), repo, branch)
		return err
	})
}

func (d *DB) DeleteAllSubscriptions(convID string, repo string) error {
	return d.runTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM subscriptions
			WHERE (conv_id = ? AND repo = ?)
		`, shortConvID(convID), repo)
		return err
	})
}

func (d *DB) GetSubscribedConvs(repo string, branch string) (res []string, err error) {
	rows, err := d.db.Query(`
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
	row := d.db.QueryRow(`
		SELECT conv_id
		FROM subscriptions
		WHERE (conv_id = ? AND repo = ? AND branch = ?)
		LIMIT 1
	`, convID, repo, branch)
	var rowRes string
	if err := row.Scan(&rowRes); err != nil {
		if err != sql.ErrNoRows {
			return false, err
		}
		return false, nil
	}
	return true, nil
}