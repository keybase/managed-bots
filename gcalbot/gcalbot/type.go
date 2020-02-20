package gcalbot

import (
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
)

type OAuthRequest struct {
	KeybaseUsername string
	AccountNickname string
	KeybaseConvID   chat1.ConvIDStr
	IsComplete      bool
}

type Account struct {
	KeybaseUsername string
	AccountNickname string
	Token           oauth2.Token
}

type Channel struct {
	ChannelID     string
	CalendarID    string
	ResourceID    string
	Expiry        time.Time
	NextSyncToken string
}

type ChannelAndAccount struct {
	Channel Channel
	Account Account
}

type SubscriptionType string

const (
	SubscriptionTypeInvite   SubscriptionType = "invite"
	SubscriptionTypeReminder SubscriptionType = "reminder"
)

type Subscription struct {
	CalendarID     string
	KeybaseConvID  chat1.ConvIDStr
	DurationBefore time.Duration
	Type           SubscriptionType
}

type SubscriptionAndAccount struct {
	Subscription Subscription
	Account      Account
}

type Invite struct {
	CalendarID string
	EventID    string
	MessageID  chat1.MessageID
}

type ReminderScheduler interface {
	UpdateOrCreateReminderEvent(
		account *Account,
		subscription *Subscription,
		event *calendar.Event,
	) error
	AddSubscription(account *Account, subscription Subscription)
	RemoveSubscription(account *Account, subscription Subscription)
}

type DaysToSendType string

const (
	DaysToSendEveryday DaysToSendType = "everyday"
	DaysToSendMonToFri DaysToSendType = "monday through friday"
	DaysToSendSatToThu DaysToSendType = "sunday through thursday"
)

type ScheduleToSendType string

const (
	ScheduleToSendToday    ScheduleToSendType = "today"
	ScheduleToSendTomorrow ScheduleToSendType = "tomorrow"
)

type DailyScheduleSubscription struct {
	CalendarID           string
	KeybaseConvID        chat1.ConvIDStr
	DaysToSend           DaysToSendType
	ScheduleToSend       ScheduleToSendType
	NotificationDuration time.Duration
}

type AggregatedDailyScheduleSubscription struct {
	CalendarIDs          []string
	KeybaseConvID        chat1.ConvIDStr
	DaysToSend           DaysToSendType
	ScheduleToSend       ScheduleToSendType
	NotificationDuration time.Duration
	Account              Account
}
