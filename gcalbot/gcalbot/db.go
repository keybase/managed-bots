package gcalbot

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"

	"golang.org/x/oauth2"

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

func (d *DB) InsertAccount(account Account) error {
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
	switch err {
	case sql.ErrNoRows:
		return nil, nil
	case nil:
		return account, nil
	default:
		return nil, err
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
	if err != nil {
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

func (d *DB) InsertChannel(channel Channel) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO channel
				(channel_id, account_id, calendar_id, resource_id, expiry, next_sync_token)
				VALUES (?, ?, ?, ?, ?, ?)
		`, channel.ChannelID, channel.AccountID, channel.CalendarID, channel.ResourceID, channel.Expiry, channel.NextSyncToken)
		return err
	})
}

func (d *DB) UpdateChannel(oldChannelID, newChannelID string, expiry time.Time) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			UPDATE channel
				SET channel_id = ?, expiry = ?
				WHERE channel_id = ?
		`, newChannelID, expiry, oldChannelID)
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
	switch err {
	case sql.ErrNoRows:
		return nil, nil
	case nil:
		channel.Expiry = time.Unix(expiry, 0)
		return channel, nil
	default:
		return nil, err
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
	switch err {
	case sql.ErrNoRows:
		return nil, nil
	case nil:
		channel.Expiry = time.Unix(expiry, 0)
		return channel, nil
	default:
		return nil, err
	}
}

func (d *DB) GetChannelListByAccountID(accountID string) (channels []*Channel, err error) {
	rows, err := d.DB.Query(`
		SELECT channel_id, account_id, calendar_id, resource_id, ROUND(UNIX_TIMESTAMP(expiry)), next_sync_token FROM channel
			WHERE account_id = ?
	`, accountID)
	if err != nil {
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

func (d *DB) GetExpiringChannelList() (channels []*Channel, err error) {
	// query all channels that are expiring in less than a day
	rows, err := d.DB.Query(`
		SELECT channel_id, account_id, calendar_id, resource_id, ROUND(UNIX_TIMESTAMP(expiry)), next_sync_token FROM channel
			WHERE expiry < DATE_ADD(NOW(), INTERVAL 1 DAY)
	`)
	if err != nil {
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
	SubscriptionTypeInvite   SubscriptionType = "invite"
	SubscriptionTypeReminder SubscriptionType = "reminder"
)

type Subscription struct {
	AccountID      string
	CalendarID     string
	KeybaseConvID  chat1.ConvIDStr
	DurationBefore time.Duration
	Type           SubscriptionType
}

type AggregatedReminderSubscription struct {
	Subscription
	Account        Account
	DurationBefore []time.Duration // aggregate MinutesBefore into an array
	Token          *oauth2.Token
}

func (d *DB) InsertSubscription(subscription Subscription) error {
	minutesBefore := GetMinutesFromDuration(subscription.DurationBefore)
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT into subscription
				(account_id, calendar_id, keybase_conv_id, minutes_before, type)
				VALUES (?, ?, ?, ?, ?)
		`, subscription.AccountID, subscription.CalendarID, subscription.KeybaseConvID,
			minutesBefore, subscription.Type)
		return err
	})
}

func (d *DB) ExistsSubscription(subscription Subscription) (exists bool, err error) {
	minutesBefore := GetMinutesFromDuration(subscription.DurationBefore)
	row := d.DB.QueryRow(`
		SELECT EXISTS(
		    SELECT * FROM subscription WHERE account_id = ? AND calendar_id = ? AND keybase_conv_id = ? AND minutes_before = ? AND type = ?
	)`, subscription.AccountID, subscription.CalendarID, subscription.KeybaseConvID, minutesBefore, subscription.Type)
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

func (d *DB) GetAggregatedReminderSubscriptions() (reminders []*AggregatedReminderSubscription, err error) {
	row, err := d.DB.Query(`
		SELECT
		       keybase_username, account_nickname, -- account
		       access_token, token_type, refresh_token, ROUND(UNIX_TIMESTAMP(expiry)), -- token
		       subscription.account_id, calendar_id, keybase_conv_id, GROUP_CONCAT(minutes_before), type -- subscription
		FROM subscription
		JOIN oauth ON subscription.account_id = oauth.identifier
		JOIN account ON subscription.account_id = account.account_id
		WHERE subscription.type = ?
		GROUP BY subscription.calendar_id
	`, SubscriptionTypeReminder)
	if err != nil {
		return nil, err
	}
	for row.Next() {
		var reminder AggregatedReminderSubscription
		reminder.Token = &oauth2.Token{}
		var expiry int64
		var minutesBeforeBytes []byte
		err = row.Scan(&reminder.Account.KeybaseUsername, &reminder.Account.AccountNickname,
			&reminder.Token.AccessToken, &reminder.Token.TokenType, &reminder.Token.RefreshToken, &expiry,
			&reminder.AccountID, &reminder.CalendarID, &reminder.KeybaseConvID, &minutesBeforeBytes, &reminder.Type)
		if err != nil {
			return nil, err
		}
		// parse MinutesBefore from GROUP_CONCAT bytes
		minutesBeforeStrings := strings.Split(string(minutesBeforeBytes), ",")
		reminder.DurationBefore = make([]time.Duration, len(minutesBeforeStrings))
		for index, minutesBeforeItem := range minutesBeforeStrings {
			minutesBefore, err := strconv.Atoi(minutesBeforeItem)
			if err != nil {
				return nil, err
			}
			reminder.DurationBefore[index] = GetDurationFromMinutes(minutesBefore)
		}
		reminder.Account.AccountID = reminder.AccountID
		reminder.Token.Expiry = time.Unix(expiry, 0)
		reminders = append(reminders, &reminder)
	}
	return reminders, nil
}

func (d *DB) GetReminderDurationBeforeList(accountID, calendarID string, keybaseConvID chat1.ConvIDStr) (durationBeforeList []time.Duration, err error) {
	rows, err := d.DB.Query(`
		SELECT minutes_before
			FROM subscription
			WHERE account_id = ? AND calendar_id = ? AND keybase_conv_id = ? AND type = ?
			ORDER BY minutes_before
	`, accountID, calendarID, keybaseConvID, SubscriptionTypeReminder)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var minutesBefore int
		err = rows.Scan(&minutesBefore)
		if err != nil {
			return nil, err
		}
		durationBeforeList = append(durationBeforeList, GetDurationFromMinutes(minutesBefore))
	}
	return durationBeforeList, nil
}

func (d *DB) DeleteSubscription(subscription Subscription) (exists bool, err error) {
	minutesBefore := GetMinutesFromDuration(subscription.DurationBefore)
	err = d.RunTxn(func(tx *sql.Tx) error {
		res, err := tx.Exec(`
			DELETE from subscription
				WHERE account_id = ? AND calendar_id = ? AND keybase_conv_id = ? AND minutes_before = ? AND type = ?
		`, subscription.AccountID, subscription.CalendarID, subscription.KeybaseConvID,
			minutesBefore, subscription.Type)
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

// Invite

type Invite struct {
	AccountID       string
	CalendarID      string
	EventID         string
	KeybaseUsername string
	MessageID       uint
}

func (d *DB) InsertInvite(invite Invite) error {
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
	switch err {
	case sql.ErrNoRows:
		return nil, nil
	case nil:
		return invite, nil
	default:
		return nil, err
	}
}
