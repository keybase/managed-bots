package elastiwatch

import (
	"context"
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

func (d *DB) Create(ctx context.Context, regex, author string) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO deferrals (regex, author, ctime) VALUES (?, ?, NOW())
	`, regex, author)
	return err
}

type Deferral struct {
	ID     int
	Regex  string
	Author string
	Ctime  time.Time
}

func (d *DB) List(ctx context.Context) (res []Deferral, err error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, regex, author, ctime FROM deferrals
	`)
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var def Deferral
		if err := rows.Scan(&def.ID, &def.Regex, &def.Author, &def.Ctime); err != nil {
			return res, err
		}
		res = append(res, def)
	}
	return res, rows.Err()
}

func (d *DB) Remove(ctx context.Context, id int) error {
	_, err := d.ExecContext(ctx, `DELETE FROM deferrals WHERE id = ?`, id)
	return err
}
