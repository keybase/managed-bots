package gcalbot

import (
	"database/sql"
	"strings"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"

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

// OAuth state
func (d *DB) GetState(state string) (*OAuthRequest, error) {
	var oauthState OAuthRequest
	row := d.DB.QueryRow(`
		SELECT keybase_username, account_nickname, keybase_conv_id, is_complete
		FROM oauth_state
		WHERE state = ?
	`, state)
	err := row.Scan(&oauthState.KeybaseUsername, &oauthState.AccountNickname, &oauthState.KeybaseConvID,
		&oauthState.IsComplete)
	switch err {
	case nil:
		return &oauthState, nil
	case sql.ErrNoRows:
		return nil, nil
	default:
		return nil, err
	}
}

func (d *DB) PutState(state string, oauthState OAuthRequest) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO oauth_state
			(state, keybase_username, account_nickname, keybase_conv_id)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
			keybase_username=VALUES(keybase_username),
			account_nickname=VALUES(account_nickname),
			keybase_conv_id=VALUES(keybase_conv_id)
		`, state, oauthState.KeybaseUsername, oauthState.AccountNickname, oauthState.KeybaseConvID)
		return err
	})
	return err
}

func (d *DB) CompleteState(state string) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			UPDATE oauth_state
			SET is_complete = true
			WHERE state = ?
		`, state)
		return err
	})
	return err
}

// Account
func (d *DB) InsertAccount(account Account) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO account
			(keybase_username, account_nickname, access_token, token_type, refresh_token, expiry, ctime, mtime)
			VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())
			ON DUPLICATE KEY UPDATE 
			access_token=VALUES(access_token),
			refresh_token=VALUES(refresh_token),
			expiry=VALUES(expiry),
			mtime=VALUES(mtime)
		`, account.KeybaseUsername, account.AccountNickname, account.Token.AccessToken, account.Token.TokenType,
			account.Token.RefreshToken, account.Token.Expiry)
		return err
	})
}

func (d *DB) GetAccount(keybaseUsername, accountNickname string) (account *Account, err error) {
	account = &Account{}
	var expiry int64
	row := d.DB.QueryRow(`
		SELECT keybase_username, account_nickname, access_token, token_type, refresh_token, ROUND(UNIX_TIMESTAMP(expiry))
		FROM account
		WHERE keybase_username = ? AND account_nickname = ?
	`, keybaseUsername, accountNickname)
	err = row.Scan(&account.KeybaseUsername, &account.AccountNickname, &account.Token.AccessToken,
		&account.Token.TokenType, &account.Token.RefreshToken, &expiry)
	switch err {
	case sql.ErrNoRows:
		return nil, nil
	case nil:
		account.Token.Expiry = time.Unix(expiry, 0)
		return account, nil
	default:
		return nil, err
	}
}

func (d *DB) DeleteAccount(keybaseUsername, accountNickname string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		// remove subscriptions first due to foreign key constraint
		_, err := tx.Exec(`
			DELETE FROM subscription
			WHERE keybase_username = ? AND account_nickname = ?
		`, keybaseUsername, accountNickname)
		if err != nil {
			return err
		}
		// remove account (and cascading remove associated channels and invites)
		_, err = tx.Exec(`
			DELETE FROM account
			WHERE keybase_username = ? AND account_nickname = ?
		`, keybaseUsername, accountNickname)
		return err
	})
}

func (d *DB) ExistsAccount(keybaseUsername string, accountNickname string) (exists bool, err error) {
	row := d.DB.QueryRow(`
		SELECT EXISTS(SELECT * FROM account WHERE keybase_username = ? AND account_nickname = ?)
	`, keybaseUsername, accountNickname)
	err = row.Scan(&exists)
	return exists, err
}

func (d *DB) GetAccountListForUsername(keybaseUsername string) (accounts []*Account, err error) {
	rows, err := d.DB.Query(`
		SELECT keybase_username, account_nickname, access_token, token_type, refresh_token, ROUND(UNIX_TIMESTAMP(expiry))
		FROM account
		WHERE keybase_username = ?
		ORDER BY account_nickname
	`, keybaseUsername)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var account Account
		var expiry int64
		err = rows.Scan(&account.KeybaseUsername, &account.AccountNickname, &account.Token.AccessToken,
			&account.Token.TokenType, &account.Token.RefreshToken, &expiry)
		account.Token.Expiry = time.Unix(expiry, 0)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, &account)
	}
	return accounts, nil
}

// Channel
func (d *DB) InsertChannel(account *Account, channel Channel) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO channel
			(channel_id, keybase_username, account_nickname, calendar_id, resource_id, expiry, next_sync_token)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, channel.ChannelID, account.KeybaseUsername, account.AccountNickname, channel.CalendarID, channel.ResourceID,
			channel.Expiry, channel.NextSyncToken)
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

func (d *DB) GetChannel(account *Account, calendarID string) (channel *Channel, err error) {
	channel = &Channel{}
	var expiry int64
	row := d.DB.QueryRow(`
		SELECT channel_id, calendar_id, resource_id, ROUND(UNIX_TIMESTAMP(channel.expiry)), next_sync_token
		FROM channel
		WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ?
	`, account.KeybaseUsername, account.AccountNickname, calendarID)
	err = row.Scan(&channel.ChannelID, &channel.CalendarID, &channel.ResourceID, &expiry, &channel.NextSyncToken)
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

func (d *DB) GetChannelAndAccountByID(channelID string) (channel *Channel, account *Account, err error) {
	channel = &Channel{}
	account = &Account{}
	var channelExpiry int64
	var tokenExpiry int64
	row := d.DB.QueryRow(`
		SELECT
		    channel_id, calendar_id, resource_id, ROUND(UNIX_TIMESTAMP(channel.expiry)), next_sync_token,
			account.keybase_username, account.account_nickname, access_token, token_type, refresh_token, ROUND(UNIX_TIMESTAMP(account.expiry))
		FROM channel
		JOIN account USING(keybase_username, account_nickname)
		WHERE channel_id = ?
	`, channelID)
	err = row.Scan(&channel.ChannelID, &channel.CalendarID, &channel.ResourceID, &channelExpiry, &channel.NextSyncToken,
		&account.KeybaseUsername, &account.AccountNickname, &account.Token.AccessToken, &account.Token.TokenType,
		&account.Token.RefreshToken, &tokenExpiry)
	switch err {
	case sql.ErrNoRows:
		return nil, nil, nil
	case nil:
		channel.Expiry = time.Unix(channelExpiry, 0)
		account.Token.Expiry = time.Unix(tokenExpiry, 0)
		return channel, account, nil
	default:
		return nil, nil, err
	}
}

func (d *DB) GetChannelListByAccount(account *Account) (channels []*Channel, err error) {
	rows, err := d.DB.Query(`
		SELECT channel_id, calendar_id, resource_id, ROUND(UNIX_TIMESTAMP(expiry)), next_sync_token
		FROM channel
		WHERE keybase_username = ? AND account_nickname = ?
	`, account.KeybaseUsername, account.AccountNickname)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var channel Channel
		var expiry int64
		err = rows.Scan(&channel.ChannelID, &channel.CalendarID, &channel.ResourceID, &expiry, &channel.NextSyncToken)
		if err != nil {
			return nil, err
		}
		channel.Expiry = time.Unix(expiry, 0)
		channels = append(channels, &channel)
	}
	return channels, nil
}

func (d *DB) GetExpiringChannelAndAccountList() (pairs []*ChannelAndAccount, err error) {
	// query all channels that are expiring in less than a day
	rows, err := d.DB.Query(`
		SELECT
		    channel_id, calendar_id, resource_id, ROUND(UNIX_TIMESTAMP(channel.expiry)), next_sync_token,
			account.keybase_username, account.account_nickname, access_token, token_type, refresh_token, ROUND(UNIX_TIMESTAMP(account.expiry))
		FROM channel
		JOIN account USING(keybase_username, account_nickname)
		WHERE channel.expiry < DATE_ADD(NOW(), INTERVAL 1 DAY)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pair ChannelAndAccount
		var channelExpiry int64
		var accountExpiry int64
		err = rows.Scan(&pair.Channel.ChannelID, &pair.Channel.CalendarID, &pair.Channel.ResourceID, &channelExpiry,
			&pair.Channel.NextSyncToken,
			&pair.Account.KeybaseUsername, &pair.Account.AccountNickname, &pair.Account.Token.AccessToken,
			&pair.Account.Token.TokenType, &pair.Account.Token.RefreshToken, &accountExpiry)
		if err != nil {
			return nil, err
		}
		pair.Channel.Expiry = time.Unix(channelExpiry, 0)
		pair.Account.Token.Expiry = time.Unix(accountExpiry, 0)
		pairs = append(pairs, &pair)
	}
	return pairs, nil
}

func (d *DB) ExistsChannelByAccountAndCalendar(account *Account, calendarID string) (exists bool, err error) {
	row := d.DB.QueryRow(`
		SELECT EXISTS(SELECT * FROM channel WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ?)
	`, account.KeybaseUsername, account.AccountNickname, calendarID)
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

// Subscription
func (d *DB) InsertSubscription(account *Account, subscription Subscription) error {
	minutesBefore := GetMinutesFromDuration(subscription.DurationBefore)
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO subscription
			(keybase_username, account_nickname, calendar_id, keybase_conv_id, minutes_before, type)
			VALUES (?, ?, ?, ?, ?, ?)
		`, account.KeybaseUsername, account.AccountNickname, subscription.CalendarID,
			subscription.KeybaseConvID, minutesBefore, subscription.Type)
		return err
	})
}

func (d *DB) ExistsSubscription(account *Account, subscription Subscription) (exists bool, err error) {
	minutesBefore := GetMinutesFromDuration(subscription.DurationBefore)
	row := d.DB.QueryRow(`
		SELECT EXISTS(
		    SELECT *
		    FROM subscription
		    WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ? AND keybase_conv_id = ? AND
		    	  minutes_before = ? AND type = ?
		)
	`, account.KeybaseUsername, account.AccountNickname, subscription.CalendarID, subscription.KeybaseConvID,
		minutesBefore, subscription.Type)
	err = row.Scan(&exists)
	return exists, err
}

func (d *DB) CountSubscriptionsByAccountAndCalender(account *Account, calendarID string) (count int, err error) {
	row := d.DB.QueryRow(`
		SELECT COUNT(*) FROM subscription WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ?
	`, account.KeybaseUsername, account.AccountNickname, calendarID)
	err = row.Scan(&count)
	return count, err
}

func (d *DB) GetReminderSubscriptionAndAccountPairs() (pairs []*SubscriptionAndAccount, err error) {
	row, err := d.DB.Query(`
		SELECT
		       calendar_id, keybase_conv_id, minutes_before, type, -- subscription
		       account.keybase_username, account.account_nickname, access_token, token_type, refresh_token, ROUND(UNIX_TIMESTAMP(expiry)) -- account
		FROM subscription
		JOIN account USING(keybase_username, account_nickname)
		WHERE subscription.type = ?
	`, SubscriptionTypeReminder)
	if err != nil {
		return nil, err
	}
	defer row.Close()
	for row.Next() {
		var pair SubscriptionAndAccount
		var subscriptionMinutesBefore int
		var tokenExpiry int64
		err = row.Scan(&pair.Subscription.CalendarID, &pair.Subscription.KeybaseConvID, &subscriptionMinutesBefore, &pair.Subscription.Type,
			&pair.Account.KeybaseUsername, &pair.Account.AccountNickname, &pair.Account.Token.AccessToken,
			&pair.Account.Token.TokenType, &pair.Account.Token.RefreshToken, &tokenExpiry)
		if err != nil {
			return nil, err
		}
		pair.Subscription.DurationBefore = GetDurationFromMinutes(subscriptionMinutesBefore)
		pair.Account.Token.Expiry = time.Unix(tokenExpiry, 0)
		pairs = append(pairs, &pair)
	}
	return pairs, nil
}

func (d *DB) GetReminderSubscriptionsByAccountAndCalendar(
	account *Account,
	calendarID string,
	subscriptionType SubscriptionType,
) (subscriptions []*Subscription, err error) {
	row, err := d.DB.Query(`
		SELECT calendar_id, keybase_conv_id, minutes_before, type
		FROM subscription
		WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ? AND type = ?
	`, account.KeybaseUsername, account.AccountNickname, calendarID, subscriptionType)
	if err != nil {
		return nil, err
	}
	defer row.Close()
	for row.Next() {
		var subscription Subscription
		var minutesBefore int
		err = row.Scan(&subscription.CalendarID, &subscription.KeybaseConvID, &minutesBefore, &subscription.Type)
		if err != nil {
			return nil, err
		}
		subscription.DurationBefore = GetDurationFromMinutes(minutesBefore)
		subscriptions = append(subscriptions, &subscription)
	}
	return subscriptions, nil
}

func (d *DB) GetSubscriptions(account *Account, calendarID string, keybaseConvID chat1.ConvIDStr) (subscriptions []*Subscription, err error) {
	rows, err := d.DB.Query(`
		SELECT calendar_id, keybase_conv_id, minutes_before, type
		FROM subscription
		WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ? AND keybase_conv_id = ?
	`, account.KeybaseUsername, account.AccountNickname, calendarID, keybaseConvID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var subscription Subscription
		var minutesBefore int
		err = rows.Scan(&subscription.CalendarID, &subscription.KeybaseConvID, &minutesBefore, &subscription.Type)
		if err != nil {
			return nil, err
		}
		subscription.DurationBefore = GetDurationFromMinutes(minutesBefore)
		subscriptions = append(subscriptions, &subscription)
	}
	return subscriptions, nil
}

func (d *DB) DeleteSubscription(account *Account, subscription Subscription) error {
	minutesBefore := GetMinutesFromDuration(subscription.DurationBefore)
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM subscription
			WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ? AND keybase_conv_id = ? AND
				  minutes_before = ? AND type = ?
		`, account.KeybaseUsername, account.AccountNickname, subscription.CalendarID,
			subscription.KeybaseConvID, minutesBefore, subscription.Type)
		return err
	})
}

// Invite
func (d *DB) InsertInvite(account *Account, invite Invite) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO invite
			(keybase_username, account_nickname, calendar_id, event_id, message_id)
			VALUES (?, ?, ?, ?, ?)
		`, account.KeybaseUsername, account.AccountNickname, invite.CalendarID, invite.EventID, invite.MessageID)
		return err
	})
}

func (d *DB) ExistsInvite(account *Account, calendarID, eventID string) (exists bool, err error) {
	row := d.DB.QueryRow(`
		SELECT EXISTS(
			SELECT * FROM invite WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ? AND event_id = ?
		)
	`, account.KeybaseUsername, account.AccountNickname, calendarID, eventID)
	err = row.Scan(&exists)
	return exists, err
}

func (d *DB) GetInviteAndAccountByUserMessage(keybaseUsername string, messageID chat1.MessageID) (invite *Invite, account *Account, err error) {
	invite = &Invite{}
	account = &Account{}
	var expiry int64
	row := d.DB.QueryRow(`
		SELECT
			calendar_id, event_id, message_id,
			account.keybase_username, account.account_nickname, access_token, token_type, refresh_token, ROUND(UNIX_TIMESTAMP(expiry))
		FROM invite
		JOIN account USING(keybase_username, account_nickname)
		WHERE invite.keybase_username = ? and message_id = ?
	`, keybaseUsername, messageID)
	err = row.Scan(&invite.CalendarID, &invite.EventID, &invite.MessageID,
		&account.KeybaseUsername, &account.AccountNickname, &account.Token.AccessToken, &account.Token.TokenType,
		&account.Token.RefreshToken, &expiry)
	switch err {
	case sql.ErrNoRows:
		return nil, nil, nil
	case nil:
		account.Token.Expiry = time.Unix(expiry, 0)
		return invite, account, nil
	default:
		return nil, nil, err
	}
}

// Daily Schedule Subscription
func (d *DB) InsertDailyScheduleSubscription(account *Account, subscription DailyScheduleSubscription) error {
	notificationDuration := GetMinutesFromDuration(subscription.NotificationDuration)
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO daily_schedule_subscription
			(keybase_username, account_nickname, keybase_conv_id, days_to_send, schedule_to_send, notification_duration)
			VALUES (?, ?, ?, ?, ?, ?)
		`, account.KeybaseUsername, account.AccountNickname, subscription.KeybaseConvID, subscription.DaysToSend,
			subscription.ScheduleToSend, notificationDuration)
		return err
	})
}

func (d *DB) AddCalendarToDailyScheduleSubscription(account *Account, keybaseConvID chat1.ConvIDStr, calendarID string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO daily_schedule_subscription_calendar
			(keybase_username, account_nickname, keybase_conv_id, calendar_id)
			VALUES (?, ?, ?, ?)
		`, account.KeybaseUsername, account.AccountNickname, keybaseConvID, calendarID)
		return err
	})
}

func (d *DB) GetAggregatedDailyScheduleSubscriptionByDuration(duration time.Duration) (subscriptions []*AggregatedDailyScheduleSubscription, err error) {
	durationMinutes := GetMinutesFromDuration(duration)
	row, err := d.DB.Query(`
		SELECT
			GROUP_CONCAT(calendar_id) as calendar_ids, daily_schedule_subscription.keybase_conv_id, days_to_send, schedule_to_send, notification_duration,
			account.keybase_username, account.account_nickname, access_token, token_type, refresh_token, ROUND(UNIX_TIMESTAMP(expiry))
		FROM daily_schedule_subscription
		JOIN account USING(keybase_username, account_nickname)
		JOIN daily_schedule_subscription_calendar USING(keybase_username, account_nickname, keybase_conv_id)
		WHERE notification_duration = ?
		GROUP BY keybase_username, account_nickname, keybase_conv_id
	`, durationMinutes)
	if err != nil {
		return nil, err
	}
	defer row.Close()
	for row.Next() {
		var pair AggregatedDailyScheduleSubscription
		var concatCalendarIDs string
		var notificationDuration int
		var tokenExpiry int64
		err = row.Scan(&concatCalendarIDs, &pair.KeybaseConvID, &pair.DaysToSend, &pair.ScheduleToSend, &notificationDuration,
			&pair.Account.KeybaseUsername, &pair.Account.AccountNickname, &pair.Account.Token.AccessToken, &pair.Account.Token.TokenType,
			&pair.Account.Token.RefreshToken, &tokenExpiry)
		if err != nil {
			return nil, err
		}
		pair.CalendarIDs = strings.Split(concatCalendarIDs, ",")
		pair.NotificationDuration = GetDurationFromMinutes(notificationDuration)
		pair.Account.Token.Expiry = time.Unix(tokenExpiry, 0)
		subscriptions = append(subscriptions, &pair)
	}
	return subscriptions, nil
}

func (d *DB) RemoveCalendarFromDailyScheduleSubscription(account *Account, keybaseConvID chat1.ConvIDStr, calendarID string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM daily_schedule_subscription_calendar
			WHERE keybase_username = ? AND account_nickname = ? AND keybase_conv_id = ? AND calendar_id = ?
		`, account.KeybaseUsername, account.AccountNickname, keybaseConvID, calendarID)
		return err
	})
}

func (d *DB) DeleteDailyScheduleSubscription(account *Account, keybaseConvID chat1.ConvIDStr) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM daily_schedule_subscription
			WHERE keybase_username = ? AND account_nickname = ? AND keybase_conv_id = ?
		`, account.KeybaseUsername, account.AccountNickname, keybaseConvID)
		return err
	})
}
