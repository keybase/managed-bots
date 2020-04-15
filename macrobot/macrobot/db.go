package macrobot

import (
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

func (d *DB) Create(channel chat1.ChatChannel, macroName, macroMessage string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO macro
			(channel_name, macro_name, macro_message)
			VALUES
			(?, ?, ?)
			ON DUPLICATE KEY UPDATE
			macro_message=VALUES(macro_message)
		`, channel.Name, macroName, macroMessage)
		return err
	})
}

func (d *DB) Get(channel chat1.ChatChannel, macroName string) (message string, err error) {
	row := d.DB.QueryRow(`
		SELECT macro_message FROM macro
		WHERE channel_name = ? AND macro_name = ?
	`, channel.Name, macroName)
	err = row.Scan(&message)
	return message, err
}

type Macro struct {
	Name    string
	Message string
}

func (d *DB) List(channel chat1.ChatChannel) (list []Macro, err error) {
	rows, err := d.DB.Query(`
		SELECT macro_name, macro_message FROM macro
		WHERE channel_name = ?
		ORDER BY macro_name
	`, channel.Name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var macro Macro
		if err := rows.Scan(&macro.Name, &macro.Message); err != nil {
			return nil, err
		}
		list = append(list, macro)
	}
	return list, nil
}

func (d *DB) Remove(channel chat1.ChatChannel, macroName string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM macro
			WHERE channel_name = ? AND macro_name = ?
		`, channel.Name, macroName)
		return err
	})
}
