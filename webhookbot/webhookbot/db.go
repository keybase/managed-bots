package webhookbot

import (
	"database/sql"
	"encoding/base64"
	"encoding/hex"

	"github.com/keybase/managed-bots/base"
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

func (d *DB) makeID(name, convID string) string {
	cdat, _ := hex.DecodeString(base.ShortConvID(convID))
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(append(cdat, []byte(name)...))
}

func (d *DB) Create(name, convID string) (string, error) {
	id := d.makeID(name, convID)
	err := d.runTxn(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
			INSERT INTO hooks
			(id, name, conv_id)
			VALUES
			(?, ?, ?)
		`, id, name, convID); err != nil {
			return err
		}
		return nil
	})
	return id, err
}

type webhook struct {
	id   string
	name string
}

func (d *DB) List(convID string) (res []webhook, err error) {
	rows, err := d.db.Query(`
		SELECT id, name FROM hooks WHERE conv_id = ?
	`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var hook webhook
		if err := rows.Scan(&hook.id, &hook.name); err != nil {
			return res, err
		}
		res = append(res, hook)
	}
	return res, nil
}

func (d *DB) Remove(name, convID string) error {
	id := d.makeID(name, convID)
	return d.runTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM hooks WHERE id = ?
		`, id)
		return err
	})
}
