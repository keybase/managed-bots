package pagerdutybot

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"

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

func (d *DB) makeID(convID chat1.ConvIDStr) (string, error) {
	secret, err := base.RandBytes(16)
	if err != nil {
		return "", err
	}
	cdat, err := hex.DecodeString(string(convID))
	if err != nil {
		return "", err
	}
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write(cdat)
	return base.URLEncoder().EncodeToString(h.Sum(nil)[:20]), nil
}

func (d *DB) Create(convID chat1.ConvIDStr) (string, error) {
	id, err := d.makeID(convID)
	if err != nil {
		return "", err
	}
	err = d.RunTxn(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
			INSERT INTO hooks
			(id, conv_id)
			VALUES
			(?, ?, ?)
		`, id, convID); err != nil {
			return err
		}
		return nil
	})
	return id, err
}

func (d *DB) GetHook(id string) (res chat1.ConvIDStr, err error) {
	row := d.DB.QueryRow(`
		SELECT conv_id FROM hooks WHERE id = ?
	`, id)
	if err := row.Scan(&res); err != nil {
		return res, err
	}
	return res, nil
}

func (d *DB) ListIDs(convID chat1.ConvIDStr) (res []string, err error) {
	rows, err := d.DB.Query(`
		SELECT id FROM hooks WHERE conv_id = ?
	`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return res, err
		}
		res = append(res, id)
	}
	return res, nil
}
