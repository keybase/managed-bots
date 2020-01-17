package gcalbot

import (
	"database/sql"
	"time"

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

// Account

type Account struct {
	KeybaseUsername string
	AccountNickname string
	AccountID       string
}

func (d *DB) InsertAccount(account *Account) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO account
				(keybase_username, account_nickname, account_id)
				VALUES (?, ?, ?)
		`, account.KeybaseUsername, account.AccountNickname, account.AccountID)
		return err
	})
}

func (d *DB) GetAccountByAccountID(accountID string) (account *Account, err error) {
	account = &Account{}
	row := d.DB.QueryRow(`
		SELECT keybase_username, account_nickname, account_id FROM account
			WHERE account_id = ?
	`, accountID)
	err = row.Scan(&account.KeybaseUsername, &account.AccountNickname, &account.AccountID)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else {
		return account, nil
	}
}

func (d *DB) ExistsAccountForUsernameAndNickname(keybaseUsername string, accountNickname string) (exists bool, err error) {
	row := d.DB.QueryRow(`
		SELECT EXISTS(SELECT * FROM account WHERE keybase_username = ? AND account_nickname = ?)
	`, keybaseUsername, accountNickname)
	err = row.Scan(&exists)
	return exists, err
}

func (d *DB) GetAccountNicknameListForUsername(keybaseUsername string) (accounts []string, err error) {
	rows, err := d.DB.Query(`
		SELECT account_nickname
			FROM account
			WHERE keybase_username = ?
			ORDER BY account_nickname
	`, keybaseUsername)
	if err == sql.ErrNoRows {
		return nil, nil
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

func (d *DB) DeleteAccountByAccountID(accountID string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE from invite
				WHERE account_id = ?
		`, accountID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`
			DELETE from subscription
				WHERE account_id = ?
		`, accountID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`
			DELETE from channel
				WHERE account_id = ?
		`, accountID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`
			DELETE from oauth
				WHERE identifier = ?
		`, accountID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`
			DELETE from account
				WHERE account_id = ?
		`, accountID)
		return err
	})
}

// Channel

type Channel struct {
	ChannelID     string
	AccountID     string
	CalendarID    string
	ResourceID    string
	Expiry        time.Time
	NextSyncToken string
}

func (d *DB) InsertChannel(channel *Channel) error {
	// TODO(marcel): should I fix the timestamp?
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO channel
				(channel_id, account_id, calendar_id, resource_id, expiry, next_sync_token)
				VALUES (?, ?, ?, ?, ?, ?)
		`, channel.ChannelID, channel.AccountID, channel.CalendarID, channel.ResourceID, channel.Expiry, channel.NextSyncToken)
		return err
	})
}

func (d *DB) GetChannelByAccountAndCalendarID(accountID, calendarID string) (channel *Channel, err error) {
	channel = &Channel{}
	var expiry int64
	row := d.DB.QueryRow(`
		SELECT channel_id, account_id, calendar_id, resource_id, ROUND(UNIX_TIMESTAMP(expiry)), next_sync_token FROM channel
		WHERE account_id = ? and calendar_id = ?
	`, accountID, calendarID)
	err = row.Scan(&channel.ChannelID, &channel.AccountID, &channel.CalendarID,
		&channel.ResourceID, &expiry, &channel.NextSyncToken)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else {
		channel.Expiry = time.Unix(expiry, 0)
		return channel, nil
	}
}

func (d *DB) GetChannelByChannelID(channelID string) (channel *Channel, err error) {
	channel = &Channel{}
	var expiry int64
	row := d.DB.QueryRow(`
		SELECT channel_id, account_id, calendar_id, resource_id, ROUND(UNIX_TIMESTAMP(expiry)), next_sync_token FROM channel
			WHERE channel_id = ?
	`, channelID)
	err = row.Scan(&channel.ChannelID, &channel.AccountID, &channel.CalendarID,
		&channel.ResourceID, &expiry, &channel.NextSyncToken)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else {
		channel.Expiry = time.Unix(expiry, 0)
		return channel, nil
	}
}

func (d *DB) GetChannelListByAccountID(accountID string) (channels []*Channel, err error) {
	rows, err := d.DB.Query(`
		SELECT channel_id, account_id, calendar_id, resource_id, ROUND(UNIX_TIMESTAMP(expiry)), next_sync_token FROM channel
		WHERE account_id = ?
	`, accountID)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	for rows.Next() {
		channel := &Channel{}
		var expiry int64
		err = rows.Scan(&channel.ChannelID, &channel.AccountID, &channel.CalendarID,
			&channel.ResourceID, &expiry, &channel.NextSyncToken)
		if err != nil {
			return nil, err
		}
		channel.Expiry = time.Unix(expiry, 0)
		channels = append(channels, channel)
	}
	return channels, nil
}

func (d *DB) ExistsChannelByAccountAndCalID(accountID, calendarID string) (exists bool, err error) {
	row := d.DB.QueryRow(`
		SELECT EXISTS(SELECT * FROM channel WHERE account_id = ? AND calendar_id = ?)
	`, accountID, calendarID)
	err = row.Scan(&exists)
	return exists, err
}

func (d *DB) DeleteChannelByChannelID(channelID string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM channel
				WHERE channel_id = ?
		`, channelID)
		return err
	})
}

func (d *DB) UpdateChannelNextSyncToken(channelID, nextSyncToken string) error {
	return d.DB.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			UPDATE channel
				SET next_sync_token = ?
				WHERE channel_id = ?
		`, nextSyncToken, channelID)
		return err
	})
}

// Subscription

type SubscriptionType string

const (
	SubscriptionTypeInvite SubscriptionType = "invite"
)

type Subscription struct {
	AccountID  string
	CalendarID string
	Type       SubscriptionType
}

func (d *DB) InsertSubscription(subscription *Subscription) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT into subscription
				(account_id, calendar_id, type)
				VALUES (?, ?, ?)
		`, subscription.AccountID, subscription.CalendarID, subscription.Type)
		return err
	})
}

func (d *DB) ExistsSubscription(subscription *Subscription) (exists bool, err error) {
	row := d.DB.QueryRow(`
		SELECT EXISTS(
		    SELECT * FROM subscription
		    	WHERE account_id = ? AND calendar_id = ? AND TYPE = ?
	)`, subscription.AccountID, subscription.CalendarID, subscription.Type)
	err = row.Scan(&exists)
	return exists, err
}

func (d *DB) CountSubscriptionsByAccountAndCalID(accountID, calendarID string) (count int, err error) {
	row := d.DB.QueryRow(`
		SELECT COUNT(*) FROM subscription WHERE account_id = ? AND calendar_id = ?
	`, accountID, calendarID)
	err = row.Scan(&count)
	return count, err
}

func (d *DB) DeleteSubscription(subscription *Subscription) (exists bool, err error) {
	err = d.RunTxn(func(tx *sql.Tx) error {
		res, err := tx.Exec(`
			DELETE from subscription
				WHERE account_id = ? AND calendar_id = ? AND type = ?
		`, subscription.AccountID, subscription.CalendarID, subscription.Type)
		if err != nil {
			return err
		}
		num, err := res.RowsAffected()
		if err != nil {
			return err
		}
		exists = num == 1
		return nil
	})
	return exists, err
}

type Invite struct {
	AccountID       string
	CalendarID      string
	EventID         string
	KeybaseUsername string
	MessageID       uint
}

func (d *DB) InsertInvite(invite *Invite) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO invite
				(account_id, calendar_id, event_id, keybase_username, message_id)
				VALUES (?, ?, ?, ?, ?)
		`, invite.AccountID, invite.CalendarID, invite.EventID, invite.KeybaseUsername, invite.MessageID)
		return err
	})
}

func (d *DB) ExistsInvite(accountID, calendarID, eventID string) (exists bool, err error) {
	row := d.DB.QueryRow(`
		SELECT EXISTS(
			SELECT * FROM invite WHERE account_id = ? AND calendar_id = ? AND event_id = ?
		)
	`, accountID, calendarID, eventID)
	err = row.Scan(&exists)
	return exists, err
}

func (d *DB) GetInviteEventByUserMessage(keybaseUsername string, messageID uint) (invite *Invite, err error) {
	invite = &Invite{}
	row := d.DB.QueryRow(`
		SELECT account_id, calendar_id, event_id, keybase_username, message_id FROM invite
			WHERE keybase_username = ? and message_id = ?
	`, keybaseUsername, messageID)
	err = row.Scan(&invite.AccountID, &invite.CalendarID, &invite.EventID, &invite.KeybaseUsername, &invite.MessageID)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else {
		return invite, nil
	}
}
