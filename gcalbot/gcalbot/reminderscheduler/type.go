package reminderscheduler

import (
	"container/list"
	"sync"
	"time"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type ReminderEvent struct {
	sync.Mutex
	EventID       string
	StartTime     time.Time
	MsgContent    string
	Subscriptions []*ReminderEventSubscriptions
}

type ReminderEventSubscriptions struct {
	KeybaseConvID chat1.ConvIDStr
	Timestamp     string
	MinutesBefore int
}

// Map of eventID (string) -> ReminderEvent (locking)
type ReminderEventMap struct {
	syncMap sync.Map
}

func (r *ReminderEventMap) Get(eventID string) (event *ReminderEvent, ok bool) {
	val, ok := r.syncMap.Load(eventID)
	if ok {
		return val.(*ReminderEvent), ok
	} else {
		return nil, ok
	}
}

func (r *ReminderEventMap) Set(eventID string, event *ReminderEvent) {
	r.syncMap.Store(eventID, event)
}

func (r *ReminderEventMap) Delete(eventID string) {
	r.syncMap.Delete(eventID)
}

type ReminderMinute struct {
	sync.Mutex
	ReminderList *list.List
}

// Map of timestamp (string) -> ReminderMinute (locking list)
type ReminderSchedule struct {
	syncMap sync.Map
}

func (r *ReminderSchedule) AddReminderToMinute(timestamp string, reminder *ReminderEvent) {
	minute, ok := r.get(timestamp)
	if !ok {
		minute = &ReminderMinute{}
		minute.ReminderList = list.New()
		r.set(timestamp, minute)
	}
	minute.Lock()
	defer minute.Unlock()
	minute.ReminderList.PushBack(reminder)
}

func (r *ReminderSchedule) ForEachReminderInMinute(
	timestamp string,
	handleEvent func(event *ReminderEvent, remove func()),
) {
	minute, ok := r.get(timestamp)
	if !ok {
		return
	}
	minute.Lock()
	defer minute.Unlock()
	for elem := minute.ReminderList.Front(); elem != nil; elem = elem.Next() {
		event := elem.Value.(*ReminderEvent)
		remove := func() { minute.ReminderList.Remove(elem) }
		handleEvent(event, remove)
	}
}

func (r *ReminderSchedule) Delete(timestamp string) {
	r.syncMap.Delete(timestamp)
}

func (r *ReminderSchedule) get(timestamp string) (minute *ReminderMinute, ok bool) {
	val, ok := r.syncMap.Load(timestamp)
	if ok {
		return val.(*ReminderMinute), ok
	} else {
		return nil, ok
	}
}

func (r *ReminderSchedule) set(timestamp string, minute *ReminderMinute) {
	r.syncMap.Store(timestamp, minute)
}
