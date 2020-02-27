package reminderscheduler

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type ReminderTimestamp string

type ReminderMessage struct {
	sync.Mutex

	EventID      string
	EventSummary string

	KeybaseUsername string
	AccountNickname string
	CalendarID      string
	KeybaseConvID   chat1.ConvIDStr

	StartTime  time.Time
	MsgContent string

	SubscriptionReminder *list.Element
	EventReminder        *list.Element
	MinuteReminders      map[time.Duration]*list.Element
}

// Subscriptions
type SubscriptionKey string

func getSubscriptionKey(
	keybaseUsername, accountNickname, calendarID string,
	keybaseConvID chat1.ConvIDStr,
) SubscriptionKey {
	return SubscriptionKey(fmt.Sprintf("%s:%s:%s:%s", keybaseUsername, accountNickname, calendarID, keybaseConvID))
}

type SubscriptionReminders struct {
	sync.Mutex
	reminderMessages map[SubscriptionKey]*list.List
}

func NewSubscriptionReminders() *SubscriptionReminders {
	return &SubscriptionReminders{
		reminderMessages: make(map[SubscriptionKey]*list.List),
	}
}

func (r *SubscriptionReminders) AddReminderMessageToSubscription(msg *ReminderMessage) {
	key := getSubscriptionKey(msg.KeybaseUsername, msg.AccountNickname, msg.CalendarID, msg.KeybaseConvID)
	r.Lock()
	defer r.Unlock()
	messages, ok := r.reminderMessages[key]
	if !ok {
		messages = list.New()
		r.reminderMessages[key] = messages
	}
	elem := messages.PushBack(msg)
	msg.SubscriptionReminder = elem
}

func (r *SubscriptionReminders) RemoveReminderMessageFromSubscription(msg *ReminderMessage) {
	key := getSubscriptionKey(msg.KeybaseUsername, msg.AccountNickname, msg.CalendarID, msg.KeybaseConvID)
	r.Lock()
	defer r.Unlock()

	messages, ok := r.reminderMessages[key]
	if ok && msg.SubscriptionReminder != nil {
		messages.Remove(msg.SubscriptionReminder)
		msg.SubscriptionReminder = nil
	}
}

func (r *SubscriptionReminders) ForEachReminderMessageInSubscription(
	keybaseUsername, accountNickname, calendarID string,
	keybaseConvID chat1.ConvIDStr,
	callback func(msg *ReminderMessage, removeReminderMessageFromSubscription func()),
) {
	key := getSubscriptionKey(keybaseUsername, accountNickname, calendarID, keybaseConvID)
	// note: this lock could be moved to the map value in order to improve performance
	r.Lock()
	defer r.Unlock()
	messages, ok := r.reminderMessages[key]
	if !ok {
		return
	}
	for elem := messages.Front(); elem != nil; elem = elem.Next() {
		reminder := elem.Value.(*ReminderMessage)
		reminder.Lock()
		remove := func() {
			messages.Remove(elem)
			reminder.SubscriptionReminder = nil
		}
		callback(reminder, remove)
		reminder.Unlock()
	}
}

// Events
type EventReminders struct {
	sync.Mutex
	reminderMessages map[string]*list.List
}

func NewEventReminders() *EventReminders {
	return &EventReminders{
		reminderMessages: make(map[string]*list.List),
	}
}

func (r *EventReminders) AddReminderMessageToEvent(msg *ReminderMessage) {
	r.Lock()
	defer r.Unlock()
	messages, ok := r.reminderMessages[msg.EventID]
	if !ok {
		messages = list.New()
		r.reminderMessages[msg.EventID] = messages
	}
	elem := messages.PushBack(msg)
	msg.EventReminder = elem
}

func (r *EventReminders) ExistsEvent(eventID string) bool {
	r.Lock()
	defer r.Unlock()
	_, ok := r.reminderMessages[eventID]
	return ok
}

func (r *EventReminders) RemoveReminderMessageFromEvent(msg *ReminderMessage) {
	r.Lock()
	defer r.Unlock()

	messages, ok := r.reminderMessages[msg.EventID]
	if ok && msg.EventReminder != nil {
		messages.Remove(msg.EventReminder)
		msg.EventReminder = nil
	}
}

func (r *EventReminders) RemoveEvent(eventID string) {
	r.Lock()
	defer r.Unlock()
	delete(r.reminderMessages, eventID)
}

func (r *EventReminders) ForEachReminderMessageInEvent(
	eventID string,
	callback func(msg *ReminderMessage),
) {
	// note: this lock could be moved to the map value in order to improve performance
	r.Lock()
	defer r.Unlock()
	messages, ok := r.reminderMessages[eventID]
	if !ok {
		return
	}
	for elem := messages.Front(); elem != nil; elem = elem.Next() {
		reminder := elem.Value.(*ReminderMessage)
		reminder.Lock()
		callback(reminder)
		reminder.Unlock()
	}
}

// Minutes
type MinuteReminders struct {
	sync.Mutex
	reminderMessages map[ReminderTimestamp]*list.List
}

func NewMinuteReminders() *MinuteReminders {
	return &MinuteReminders{
		reminderMessages: make(map[ReminderTimestamp]*list.List),
	}
}

func (r *MinuteReminders) AddReminderMessageToMinute(duration time.Duration, msg *ReminderMessage) {
	r.Lock()
	defer r.Unlock()

	minute, ok := msg.MinuteReminders[duration]
	if ok && minute != nil {
		return
	}

	timestamp := getReminderTimestamp(msg.StartTime, duration)
	messages, ok := r.reminderMessages[timestamp]
	if !ok {
		messages = list.New()
		r.reminderMessages[timestamp] = messages
	}
	minute = messages.PushBack(msg)
	msg.MinuteReminders[duration] = minute
}

func (r *MinuteReminders) RemoveReminderMessageFromAllMinutes(msg *ReminderMessage) {
	r.Lock()
	defer r.Unlock()

	for duration, minute := range msg.MinuteReminders {
		timestamp := getReminderTimestamp(msg.StartTime, duration)
		messages, ok := r.reminderMessages[timestamp]
		if ok && minute != nil {
			messages.Remove(minute)
		}
		delete(msg.MinuteReminders, duration)
	}
}

func (r *MinuteReminders) RemoveReminderMessageFromMinute(msg *ReminderMessage, duration time.Duration) {
	r.Lock()
	defer r.Unlock()

	minute, ok := msg.MinuteReminders[duration]
	if !ok {
		return
	}

	timestamp := getReminderTimestamp(msg.StartTime, duration)
	messages, ok := r.reminderMessages[timestamp]
	if ok && minute != nil {
		messages.Remove(minute)
	}

	delete(msg.MinuteReminders, duration)
}

func (r *MinuteReminders) RemoveMinute(timestamp ReminderTimestamp) {
	r.Lock()
	defer r.Unlock()
	delete(r.reminderMessages, timestamp)
}

func (r *MinuteReminders) ForEachReminderMessageInMinute(
	timestamp ReminderTimestamp,
	callback func(msg *ReminderMessage),
) {
	// note: this lock could be moved to the map value in order to improve performance
	r.Lock()
	defer r.Unlock()
	messages, ok := r.reminderMessages[timestamp]
	if !ok {
		return
	}
	for elem := messages.Front(); elem != nil; elem = elem.Next() {
		reminder := elem.Value.(*ReminderMessage)
		reminder.Lock()
		callback(reminder)
		reminder.Unlock()
	}
}
