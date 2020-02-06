package elastiwatch

import (
	"database/sql"
	"time"

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

func (d *DB) Create(regex, author string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO deferrals (regex, author, ctime) VALUES (?, ?, NOW())
		`, regex, author)
		return err
	})
}

type deferral struct {
	id     int
	regex  string
	author string
	ctime  time.Time
}

func (d *DB) List() (res []deferral, err error) {
	rows, err := d.Query(`
		SELECT id, regex, author, ctime FROM deferrals
	`)
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var def deferral
		if err := rows.Scan(&def.id, &def.regex, &def.author, &def.ctime); err != nil {
			return res, err
		}
		res = append(res, def)
	}
	return res, nil
}

func (d *DB) Remove(id int) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM deferrals WHERE id = ?
		`, id)
		return err
	})
}
