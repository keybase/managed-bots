package pollbot

import (
	"context"
	"database/sql"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type TallyResult struct {
	choice int
	votes  int
}

type Tally []TallyResult

type DB struct {
	*base.DB
}

func NewDB(db *sql.DB) *DB {
	return &DB{
		DB: base.NewDB(db),
	}
}

func (d *DB) CreatePoll(ctx context.Context, id string, convID chat1.ConvIDStr, msgID chat1.MessageID, resultMsgID chat1.MessageID, numChoices int) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO polls
		(id, conv_id, msg_id, result_msg_id, choices)
		VALUES
		(?, ?, ?, ?, ?)
	`, id, convID, msgID, resultMsgID, numChoices)
	return err
}

func (d *DB) GetPollInfo(ctx context.Context, id string) (convID chat1.ConvIDStr, resultMsgID chat1.MessageID, numChoices int, err error) {
	row := d.QueryRowContext(ctx, `
		SELECT conv_id, result_msg_id, choices
		FROM polls
		WHERE id = ?
	`, id)
	if err := row.Scan(&convID, &resultMsgID, &numChoices); err != nil {
		return convID, resultMsgID, numChoices, err
	}
	return convID, resultMsgID, numChoices, nil
}

func (d *DB) GetTally(ctx context.Context, id string) (res Tally, err error) {
	rows, err := d.QueryContext(ctx, `
		SELECT choice, count(*)
		FROM votes
		WHERE id = ?
		GROUP BY 1 ORDER BY 2 DESC
	`, id)
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var tres TallyResult
		if err := rows.Scan(&tres.choice, &tres.votes); err != nil {
			return res, err
		}
		res = append(res, tres)
	}
	return res, rows.Err()
}

func (d *DB) CastVote(ctx context.Context, username string, vote Vote) error {
	_, err := d.ExecContext(ctx, `
		REPLACE INTO votes
		(id, username, choice)
		VALUES
		(?, ?, ?)
	`, vote.ID, username, vote.Choice)
	return err
}
