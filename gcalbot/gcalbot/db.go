package gcalbot

import (
	"database/sql"

	"github.com/keybase/managed-bots/base"
)

type DB struct {
	*base.GoogleOAuthDB
}

func NewDB(db *sql.DB) *DB {
	return &DB{
		GoogleOAuthDB: base.NewGoogleOAuthDB(db),
	}
}

func (d *DB) GetAccountsForUser(username string) (accounts []string, err error) {
	rows, err := d.DB.Query(`SELECT nickname
		FROM account
		WHERE username = ?
		ORDER BY nickname`, username)
	if err == sql.ErrNoRows {
		return accounts, nil
	} else if err != nil {
		return nil, err
	}
	for rows.Next() {
		var account string
		err = rows.Scan(&account)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func (d *DB) ExistsAccountForUser(username string, nickname string) (exists bool, err error) {
	row := d.DB.QueryRow(`SELECT EXISTS(
		SELECT * FROM account WHERE username = ? AND nickname = ?)`,
		username, nickname)
	err = row.Scan(&exists)
	return exists, err
}

func (d *DB) InsertAccountForUser(username string, nickname string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO account
			(username, nickname)
			VALUES (?, ?)
		`, username, nickname)
		return err
	})
}

func (d *DB) DeleteAccountForUser(username string, nickname string) error {
	identifier := GetAccountIdentifier(username, nickname)
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM oauth
			WHERE identifier = ?`, identifier)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM account
			WHERE username = ? and nickname = ?`, username, nickname)
		return err
	})
}

type Channel struct {
	Username      string
	Nickname      string
	CalendarID    string
	ChannelID     string
	NextSyncToken string
}

func (d *DB) InsertChannel(channel *Channel) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO channel
			(username, nickname, calendar_id, channel_id, next_sync_token)
			VALUES (?, ?, ?, ?, ?)
		`, channel.Username, channel.Nickname, channel.CalendarID, channel.ChannelID, channel.NextSyncToken)
		return err
	})
}

func (d *DB) ExistsChannelForUser(username, nickname, calendarID string) (exists bool, err error) {
	row := d.DB.QueryRow(`SELECT EXISTS(
    	SELECT * FROM channel WHERE username = ? AND nickname = ? AND calendar_id = ?)`,
		username, nickname, calendarID)
	err = row.Scan(&exists)
	return exists, err
}

func (d *DB) GetChannelByChannelID(channelID string) (channel *Channel, err error) {
	channel = &Channel{}
	row := d.DB.QueryRow(`SELECT username, nickname, calendar_id, channel_id, next_sync_token FROM channel
		WHERE channel_id = ?`, channelID)
	err = row.Scan(&channel.Username, &channel.Nickname, &channel.CalendarID, &channel.ChannelID, &channel.NextSyncToken)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else {
		return channel, nil
	}
}

func (d *DB) UpdateChannelNextSyncToken(channelID, nextSyncToken string) error {
	return d.DB.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE channel
			SET next_sync_token = ?
			WHERE channel_id = ?
		`, nextSyncToken, channelID)
		return err
	})
}

type Invite struct {
	Username   string
	Nickname   string
	CalendarID string
	EventID    string
	MessageID  uint
}

func (d *DB) InsertInvite(invite *Invite) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO invite
			(username, nickname, calendar_id, event_id, message_id)
			VALUES (?, ?, ?, ?, ?)
		`, invite.Username, invite.Nickname, invite.CalendarID, invite.EventID, invite.MessageID)
		return err
	})
}

func (d *DB) ExistsInviteForUserEvent(username, nickname, calendarID, eventID string) (exists bool, err error) {
	row := d.DB.QueryRow(`SELECT EXISTS(
		SELECT * FROM invite WHERE username = ? AND nickname = ? AND calendar_id = ? AND event_id = ?)`,
		username, nickname, calendarID, eventID)
	err = row.Scan(&exists)
	return exists, err
}

func (d *DB) GetInviteEventByUserMessage(username string, messageID uint) (invite *Invite, err error) {
	invite = &Invite{}
	row := d.DB.QueryRow(`SELECT username, nickname, calendar_id, event_id, message_id FROM invite
		WHERE username = ? and message_id = ?
	`, username, messageID)
	err = row.Scan(&invite.Username, &invite.Nickname, &invite.CalendarID, &invite.EventID, &invite.MessageID)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else {
		return invite, nil
	}
}
