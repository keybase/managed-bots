package base

import (
	"database/sql"
	"fmt"
)

type DB struct {
	*sql.DB
}

func NewDB(db *sql.DB) *DB {
	return &DB{
		DB: db,
	}
}

func (d *DB) RunTxn(fn func(tx *sql.Tx) error) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			fmt.Printf("unable to rollback: %v", rerr)
		}
		return err
	}
	return tx.Commit()
}
