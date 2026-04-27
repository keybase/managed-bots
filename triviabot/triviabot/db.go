package triviabot

import (
	"context"
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

func (d *DB) RecordAnswer(ctx context.Context, convID chat1.ConvIDStr, username string, pointAdjust int, isCorrect bool) error {
	correct := 0
	incorrect := 0
	if isCorrect {
		correct = 1
	} else {
		incorrect = 1
	}
	_, err := d.ExecContext(ctx, `
		INSERT INTO leaderboard (conv_id, username, points, correct, incorrect)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE points=points+VALUES(points),correct=correct+VALUES(correct),
							    incorrect=incorrect+VALUES(incorrect)
	`, base.ShortConvID(convID), username, pointAdjust, correct, incorrect)
	return err
}

type TopUser struct {
	Username  string
	Points    int
	Correct   int
	Incorrect int
}

func (d *DB) TopUsers(ctx context.Context, convID chat1.ConvIDStr) (res []TopUser, err error) {
	rows, err := d.QueryContext(ctx, `
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
	return res, rows.Err()
}

func (d *DB) ResetConv(ctx context.Context, convID chat1.ConvIDStr) error {
	_, err := d.ExecContext(ctx, `DELETE FROM leaderboard WHERE conv_id = ?`, base.ShortConvID(convID))
	return err
}

func (d *DB) GetAPIToken(ctx context.Context, convID chat1.ConvIDStr) (res string, err error) {
	row := d.QueryRowContext(ctx, `
		SELECT token FROM tokens where conv_id = ?
	`, base.ShortConvID(convID))
	if err := row.Scan(&res); err != nil {
		return "", err
	}
	return res, nil
}

func (d *DB) SetAPIToken(ctx context.Context, convID chat1.ConvIDStr, token string) error {
	_, err := d.ExecContext(ctx, `REPLACE INTO tokens (conv_id, token) VALUES (?, ?)`, base.ShortConvID(convID), token)
	return err
}
