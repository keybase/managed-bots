package triviabot

import (
	"database/sql"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
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

func (d *DB) RecordAnswer(convID chat1.ConvIDStr, username string, pointAdjust int, isCorrect bool) error {
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

type TopUser struct {
	Username  string
	Points    int
	Correct   int
	Incorrect int
}

func (d *DB) TopUsers(convID chat1.ConvIDStr) (res []TopUser, err error) {
	rows, err := d.Query(`
		SELECT username, points, correct, incorrect
		FROM leaderboard
		WHERE conv_id = ?
		ORDER BY points DESC, correct DESC
		LIMIT 10
	`, base.ShortConvID(convID))
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var user TopUser
		if err := rows.Scan(&user.Username, &user.Points, &user.Correct, &user.Incorrect); err != nil {
			return res, err
		}
		res = append(res, user)
	}
	return res, nil
}

func (d *DB) ResetConv(convID chat1.ConvIDStr) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
			DELETE FROM leaderboard WHERE conv_id  = ?
		`, base.ShortConvID(convID)); err != nil {
			return err
		}
		return nil
	})
}

func (d *DB) GetAPIToken(convID chat1.ConvIDStr) (res string, err error) {
	row := d.QueryRow(`
		SELECT token FROM tokens where conv_id = ?
	`, base.ShortConvID(convID))
	if err := row.Scan(&res); err != nil {
		return "", err
	}
	return res, nil
}

func (d *DB) SetAPIToken(convID chat1.ConvIDStr, token string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
			REPLACE INTO tokens (conv_id, token) VALUES (?, ?)
		`, base.ShortConvID(convID), token); err != nil {
			return err
		}
		return nil
	})
}
