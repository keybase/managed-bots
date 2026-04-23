package gcalbot

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"

	"github.com/keybase/managed-bots/base"
)

type DB struct {
	*base.DB
	*base.DebugOutput
}

func NewDB(
	db *sql.DB,
	debugConfig *base.ChatDebugOutputConfig,
) *DB {
	return &DB{
		DB:          base.NewDB(db),
		DebugOutput: base.NewDebugOutput("DB", debugConfig),
	}
}

// OAuth state
func (d *DB) GetState(ctx context.Context, state string) (*OAuthRequest, error) {
	var oauthState OAuthRequest
	row := d.QueryRowContext(ctx, `
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

func (d *DB) PutState(ctx context.Context, state string, oauthState OAuthRequest) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO oauth_state
		(state, keybase_username, account_nickname, keybase_conv_id)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		keybase_username=VALUES(keybase_username),
		account_nickname=VALUES(account_nickname),
		keybase_conv_id=VALUES(keybase_conv_id)
	`, state, oauthState.KeybaseUsername, oauthState.AccountNickname, oauthState.KeybaseConvID)
	return err
}

func (d *DB) CompleteState(ctx context.Context, state string) error {
	_, err := d.ExecContext(ctx, `
		UPDATE oauth_state
		SET is_complete = true
		WHERE state = ?
	`, state)
	return err
}

// Account
func (d *DB) InsertAccount(ctx context.Context, account Account) error {
	_, err := d.ExecContext(ctx, `
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
}

func (d *DB) GetAccount(ctx context.Context, keybaseUsername, accountNickname string) (account *Account, err error) {
	account = &Account{}
	var expiry int64
	row := d.QueryRowContext(ctx, `
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

func (d *DB) DeleteAccount(ctx context.Context, keybaseUsername, accountNickname string) error {
	return d.RunTxnContext(ctx, func(tx *sql.Tx) error {
		// remove subscriptions first due to foreign key constraint
		_, err := tx.ExecContext(ctx, `
			DELETE FROM subscription
			WHERE keybase_username = ? AND account_nickname = ?
		`, keybaseUsername, accountNickname)
		if err != nil {
			return err
		}
		// remove account (and cascading remove associated channels and invites)
		_, err = tx.ExecContext(ctx, `
			DELETE FROM account
			WHERE keybase_username = ? AND account_nickname = ?
		`, keybaseUsername, accountNickname)
		return err
	})
}

func (d *DB) ExistsAccount(ctx context.Context, keybaseUsername string, accountNickname string) (exists bool, err error) {
	row := d.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT * FROM account WHERE keybase_username = ? AND account_nickname = ?)
	`, keybaseUsername, accountNickname)
	err = row.Scan(&exists)
	return exists, err
}

func (d *DB) GetAccountListForUsername(ctx context.Context, keybaseUsername string) (accounts []*Account, err error) {
	rows, err := d.QueryContext(ctx, `
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
	return accounts, rows.Err()
}

// Channel
func (d *DB) InsertChannel(ctx context.Context, account *Account, channel Channel) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO channel
		(channel_id, keybase_username, account_nickname, calendar_id, resource_id, expiry, next_sync_token)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, channel.ChannelID, account.KeybaseUsername, account.AccountNickname, channel.CalendarID, channel.ResourceID,
		channel.Expiry, channel.NextSyncToken)
	return err
}

func (d *DB) UpdateChannel(ctx context.Context, oldChannelID, newChannelID string, resourceID string, expiry time.Time) error {
	_, err := d.ExecContext(ctx, `
		UPDATE channel
		SET channel_id = ?, resource_id = ?, expiry = ?
		WHERE channel_id = ?
	`, newChannelID, resourceID, expiry, oldChannelID)
	return err
}

func (d *DB) UpdateChannelNextSyncToken(ctx context.Context, channelID, nextSyncToken string) error {
	_, err := d.ExecContext(ctx, `
		UPDATE channel
		SET next_sync_token = ?
		WHERE channel_id = ?
	`, nextSyncToken, channelID)
	return err
}

func (d *DB) GetChannel(ctx context.Context, account *Account, calendarID string) (channel *Channel, err error) {
	channel = &Channel{}
	var expiry int64
	row := d.QueryRowContext(ctx, `
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

func (d *DB) GetChannelAndAccountByID(ctx context.Context, channelID string) (channel *Channel, account *Account, err error) {
	channel = &Channel{}
	account = &Account{}
	var channelExpiry int64
	var tokenExpiry int64
	row := d.QueryRowContext(ctx, `
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

func (d *DB) GetChannelListByAccount(ctx context.Context, account *Account) (channels []*Channel, err error) {
	rows, err := d.QueryContext(ctx, `
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
	return channels, rows.Err()
}

func (d *DB) GetExpiringChannelAndAccountList(ctx context.Context) (pairs []*ChannelAndAccount, err error) {
	// query all channels that are expiring in less than a day
	rows, err := d.QueryContext(ctx, `
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
	return pairs, rows.Err()
}

func (d *DB) ExistsChannelByAccountAndCalendar(ctx context.Context, account *Account, calendarID string) (exists bool, err error) {
	row := d.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT * FROM channel WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ?)
	`, account.KeybaseUsername, account.AccountNickname, calendarID)
	err = row.Scan(&exists)
	return exists, err
}

func (d *DB) DeleteChannelByChannelID(ctx context.Context, channelID string) error {
	_, err := d.ExecContext(ctx, `
		DELETE FROM channel
		WHERE channel_id = ?
	`, channelID)
	return err
}

// Subscription
func (d *DB) InsertSubscription(ctx context.Context, account *Account, subscription Subscription) error {
	minutesBefore := GetMinutesFromDuration(subscription.DurationBefore)
	_, err := d.ExecContext(ctx, `
		INSERT INTO subscription
		(keybase_username, account_nickname, calendar_id, keybase_conv_id, minutes_before, type)
		VALUES (?, ?, ?, ?, ?, ?)
	`, account.KeybaseUsername, account.AccountNickname, subscription.CalendarID,
		subscription.KeybaseConvID, minutesBefore, subscription.Type)
	return err
}

func (d *DB) ExistsSubscription(ctx context.Context, account *Account, subscription Subscription) (exists bool, err error) {
	minutesBefore := GetMinutesFromDuration(subscription.DurationBefore)
	row := d.QueryRowContext(ctx, `
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

func (d *DB) CountSubscriptionsByAccountAndCalender(ctx context.Context, account *Account, calendarID string) (count int, err error) {
	row := d.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM subscription WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ?
	`, account.KeybaseUsername, account.AccountNickname, calendarID)
	err = row.Scan(&count)
	return count, err
}

func (d *DB) GetReminderSubscriptionAndAccountPairs(ctx context.Context) (pairs []*SubscriptionAndAccount, err error) {
	rows, err := d.QueryContext(ctx, `
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
	defer rows.Close()
	for rows.Next() {
		var pair SubscriptionAndAccount
		var subscriptionMinutesBefore int
		var tokenExpiry int64
		err = rows.Scan(&pair.Subscription.CalendarID, &pair.Subscription.KeybaseConvID, &subscriptionMinutesBefore, &pair.Subscription.Type,
			&pair.Account.KeybaseUsername, &pair.Account.AccountNickname, &pair.Account.Token.AccessToken,
			&pair.Account.Token.TokenType, &pair.Account.Token.RefreshToken, &tokenExpiry)
		if err != nil {
			return nil, err
		}
		pair.Subscription.DurationBefore = GetDurationFromMinutes(subscriptionMinutesBefore)
		pair.Account.Token.Expiry = time.Unix(tokenExpiry, 0)
		pairs = append(pairs, &pair)
	}
	return pairs, rows.Err()
}

func (d *DB) GetReminderSubscriptionsByAccountAndCalendar(
	ctx context.Context,
	account *Account,
	calendarID string,
	subscriptionType SubscriptionType,
) (subscriptions []*Subscription, err error) {
	rows, err := d.QueryContext(ctx, `
		SELECT calendar_id, keybase_conv_id, minutes_before, type
		FROM subscription
		WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ? AND type = ?
	`, account.KeybaseUsername, account.AccountNickname, calendarID, subscriptionType)
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
	return subscriptions, rows.Err()
}

func (d *DB) GetSubscriptions(ctx context.Context, account *Account, calendarID string, keybaseConvID chat1.ConvIDStr) (subscriptions []*Subscription, err error) {
	rows, err := d.QueryContext(ctx, `
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
	return subscriptions, rows.Err()
}

func (d *DB) DeleteSubscription(ctx context.Context, account *Account, subscription Subscription) error {
	minutesBefore := GetMinutesFromDuration(subscription.DurationBefore)
	_, err := d.ExecContext(ctx, `
		DELETE FROM subscription
		WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ? AND keybase_conv_id = ? AND
			  minutes_before = ? AND type = ?
	`, account.KeybaseUsername, account.AccountNickname, subscription.CalendarID,
		subscription.KeybaseConvID, minutesBefore, subscription.Type)
	return err
}

// Invite
func (d *DB) InsertInvite(ctx context.Context, account *Account, invite Invite) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO invite
		(keybase_username, account_nickname, calendar_id, event_id, message_id)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		message_id=message_id -- message id stays the same
	`, account.KeybaseUsername, account.AccountNickname, invite.CalendarID, invite.EventID, invite.MessageID)
	return err
}

func (d *DB) ExistsInvite(ctx context.Context, account *Account, calendarID, eventID string) (exists bool, err error) {
	row := d.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT * FROM invite WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ? AND event_id = ?
		)
	`, account.KeybaseUsername, account.AccountNickname, calendarID, eventID)
	err = row.Scan(&exists)
	return exists, err
}

func (d *DB) GetInviteAndAccountByUserMessage(ctx context.Context, keybaseUsername string, messageID chat1.MessageID) (invite *Invite, account *Account, err error) {
	invite = &Invite{}
	account = &Account{}
	var expiry int64
	row := d.QueryRowContext(ctx, `
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
func (d *DB) InsertDailyScheduleSubscription(ctx context.Context, account *Account, subscription DailyScheduleSubscription) error {
	notificationTime := GetTimeStringFromDuration(subscription.NotificationTime)
	_, err := d.ExecContext(ctx, `
		INSERT INTO daily_schedule_subscription
		(keybase_username, account_nickname, calendar_id, keybase_conv_id, timezone, days_to_send, schedule_to_send, notification_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		timezone=VALUES(timezone),
		days_to_send=VALUES(days_to_send),
		schedule_to_send=VALUES(schedule_to_send),
		notification_time=VALUES(notification_time)
	`, account.KeybaseUsername, account.AccountNickname, subscription.CalendarID, subscription.KeybaseConvID,
		subscription.Timezone.String(), subscription.DaysToSend, subscription.ScheduleToSend, notificationTime)
	return err
}

func (d *DB) GetAggregatedDailyScheduleSubscription(ctx context.Context, scheduleToSend ScheduleToSendType) (subscriptions []*AggregatedDailyScheduleSubscription, err error) {
	rows, err := d.QueryContext(ctx, `
		SELECT
			GROUP_CONCAT(calendar_id) as calendar_ids, keybase_conv_id, timezone, days_to_send, schedule_to_send, notification_time,
			account.keybase_username, account.account_nickname, access_token, token_type, refresh_token, ROUND(UNIX_TIMESTAMP(expiry))
		FROM daily_schedule_subscription
		JOIN account USING(keybase_username, account_nickname)
		WHERE schedule_to_send = ?
		GROUP BY keybase_username, account_nickname, keybase_conv_id, notification_time
	`, scheduleToSend)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pair AggregatedDailyScheduleSubscription
		var concatCalendarIDs string
		var timezone string
		var notificationTime string
		var tokenExpiry int64
		err = rows.Scan(&concatCalendarIDs, &pair.KeybaseConvID, &timezone, &pair.DaysToSend, &pair.ScheduleToSend, &notificationTime,
			&pair.Account.KeybaseUsername, &pair.Account.AccountNickname, &pair.Account.Token.AccessToken, &pair.Account.Token.TokenType,
			&pair.Account.Token.RefreshToken, &tokenExpiry)
		if err != nil {
			return nil, err
		}
		pair.CalendarIDs = strings.Split(concatCalendarIDs, ",")
		pair.Timezone, err = time.LoadLocation(timezone)
		if err != nil {
			d.Errorf("couldn't parse timezone %s: %s", timezone, err)
			continue
		}
		pair.NotificationTime, err = GetDurationFromTimeString(notificationTime)
		if err != nil {
			d.Errorf("couldn't parse time %s: %s", notificationTime, err)
			continue
		}
		pair.Account.Token.Expiry = time.Unix(tokenExpiry, 0)
		subscriptions = append(subscriptions, &pair)
	}
	return subscriptions, rows.Err()
}

func (d *DB) GetDailyScheduleSubscription(
	ctx context.Context,
	account *Account,
	calendarID string,
	keybaseConvID chat1.ConvIDStr,
) (subscription *DailyScheduleSubscription, exists bool, err error) {
	subscription = &DailyScheduleSubscription{}
	var timezone string
	var notificationTime string
	row := d.QueryRowContext(ctx, `
		SELECT calendar_id, keybase_conv_id, timezone, days_to_send, schedule_to_send, notification_time
		FROM daily_schedule_subscription
		WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ? AND keybase_conv_id = ?
	`, account.KeybaseUsername, account.AccountNickname, calendarID, keybaseConvID)
	err = row.Scan(&subscription.CalendarID, &subscription.KeybaseConvID, &timezone, &subscription.DaysToSend,
		&subscription.ScheduleToSend, &notificationTime)
	switch err {
	case nil:
		subscription.Timezone, err = time.LoadLocation(timezone)
		if err != nil {
			return nil, false, err
		}
		subscription.NotificationTime, err = GetDurationFromTimeString(notificationTime)
		if err != nil {
			return nil, false, err
		}
		return subscription, true, nil
	case sql.ErrNoRows:
		return nil, false, nil
	default:
		return nil, false, err
	}
}

func (d *DB) DeleteDailyScheduleSubscription(ctx context.Context, account *Account, calendarID string, keybaseConvID chat1.ConvIDStr) error {
	_, err := d.ExecContext(ctx, `
		DELETE FROM daily_schedule_subscription
		WHERE keybase_username = ? AND account_nickname = ? AND calendar_id = ? AND keybase_conv_id = ?
	`, account.KeybaseUsername, account.AccountNickname, calendarID, keybaseConvID)
	return err
}
