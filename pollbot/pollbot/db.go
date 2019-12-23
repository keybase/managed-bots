package pollbot

import (
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

func (d *DB) CreatePoll(convID string, msgID chat1.MessageID, resultMsgID chat1.MessageID, numChoices int) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO polls
			(conv_id, msg_id, result_msg_id, choices)
			VALUES
			(?, ?, ?, ?)
		`, base.ShortConvID(convID), msgID, resultMsgID, numChoices)
		return err
	})
}

func (d *DB) GetPollInfo(convID string, msgID chat1.MessageID) (resultMsgID chat1.MessageID, numChoices int, err error) {
	row := d.DB.QueryRow(`
		SELECT result_msg_id, choices
		FROM polls
		WHERE conv_id = ? AND msg_id = ?
	`, convID, msgID)
	if err := row.Scan(&resultMsgID, &numChoices); err != nil {
		return resultMsgID, numChoices, err
	}
	return resultMsgID, numChoices, nil
}

func (d *DB) GetTally(convID string, msgID chat1.MessageID) (res Tally, err error) {
	rows, err := d.DB.Query(`
		SELECT choice, count(*)
		FROM votes
		WHERE conv_id = ? AND msg_id = ?
		GROUP BY 1 ORDER BY 2 DESC
	`, convID, msgID)
	if err != nil {
		return res, err
	}
	for rows.Next() {
		var tres TallyResult
		if err := rows.Scan(&tres.choice, &tres.votes); err != nil {
			return res, err
		}
		res = append(res, tres)
	}
	return res, nil
}

func (d *DB) CastVote(username string, vote Vote) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			REPLACE INTO votes
			(conv_id, msg_id, username, choice)
			VALUES
			(?, ?, ?, ?)
		`, base.ShortConvID(vote.ConvID), vote.MsgID, username, vote.Choice)
		return err
	})
}
