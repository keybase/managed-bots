package triviabot

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

func (d *DB) RecordAnswer(convID string, username string, pointAdjust int, isCorrect bool) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		correct := 0
		incorrect := 0
		if isCorrect {
			correct = 1
		} else {
			incorrect = 1
		}
		if _, err := tx.Exec(`
			INSERT INTO leaderboard (conv_id, username, points, correct, incorrect)
			VALUES (?, ?, ?, ?, ?) 
			ON DUPLICATE KEY UPDATE points=points+VALUES(points),correct=correct+VALUES(correct), 
								    incorrect=incorrect+VALUES(incorrect)
		`, base.ShortConvID(convID), username, pointAdjust, correct, incorrect); err != nil {
			return err
		}
		return nil
	})
}

type topUser struct {
	username  string
	points    int
	correct   int
	incorrect int
}

func (d *DB) TopUsers(convID string) (res []topUser, err error) {
	rows, err := d.Query(`
		SELECT username, points, correct, incorrect
		FROM leaderboard
		WHERE conv_id = ?
		ORDER BY points
		LIMIT 10
	`, base.ShortConvID(convID))
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var user topUser
		if err := rows.Scan(&user.username, &user.points, &user.correct, &user.incorrect); err != nil {
			return res, err
		}
		res = append(res, user)
	}
	return res, nil
}

func (d *DB) ResetConv(convID string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
			DELETE FROM leaderboard WHERE conv_id  = ?
		`, base.ShortConvID(convID)); err != nil {
			return err
		}
		return nil
	})
}
