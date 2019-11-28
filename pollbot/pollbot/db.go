package pollbot

import (
	"database/sql"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type TallyResult struct {
	choice int
	votes  int
}

type Tally []TallyResult

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

func (d *DB) StageVote(username string, msgID chat1.MessageID, vote Vote) error {
	return d.runTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO staged_votes
			(username, msg_id, vote)
			VALUES
			(?, ?, ?)
		`, username, msgID, vote.Encode())
		return err
	})
}

func (d *DB) GetStagedVote(username string, msgID chat1.MessageID) (res Vote, err error) {
	row := d.db.QueryRow(`SELECT vote FROM staged_votes WHERE username = ? AND msg_id = ?`,
		username, msgID)
	var vstr string
	if err := row.Scan(&vstr); err != nil {
		return res, err
	}
	return NewVoteFromEncoded(vstr), nil
}

func (d *DB) CreatePoll(convID string, msgID chat1.MessageID, resultMsgID chat1.MessageID) error {
	return d.runTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO polls
			(conv_id, msg_id, result_msg_id)
			VALUES
			(?, ?, ?)
		`, convID, msgID, resultMsgID)
		return err
	})
}

func (d *DB) GetPollResultMsgID(convID string, msgID chat1.MessageID) (res chat1.MessageID, err error) {
	row := d.db.QueryRow(`SELECT result_msg_id FROM polls WHERE conv_id = ? AND msg_id = ?`, convID, msgID)
	if err := row.Scan(&res); err != nil {
		return res, err
	}
	return res, nil
}

func (d *DB) GetTally(convID string, msgID chat1.MessageID) (res Tally, err error) {
	rows, err := d.db.Query(`
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

func (d *DB) CastVote(username string, vote Vote, stagedMsgID chat1.MessageID) error {
	return d.runTxn(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
			REPLACE INTO votes
			(conv_id, msg_id, username, choice)
			VALUES
			(?, ?, ?, ?)
		`, vote.ConvID, vote.MsgID, username, vote.Choice); err != nil {
			return err
		}
		_, err := tx.Exec(`
			DELETE FROM staged_votes WHERE username = ? AND msg_id = ?
		`, username, stagedMsgID)
		return err
	})
}
