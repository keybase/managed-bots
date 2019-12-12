package base

import "database/sql"

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
		tx.Rollback()
		return err
	}
	return tx.Commit()
}
