package gcalbot

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/keybase/managed-bots/base"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
)

func (h *HTTPSrv) handleEventUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	var err error
	defer func() {
		if err != nil {
			h.Errorf("error in event update webhook: %s", err)
		}
	}()

	state := r.Header.Get("X-Goog-Resource-State")
	if state == "sync" {
		// sync header, safe to ignore
		return
	}

	channelID := r.Header.Get("X-Goog-Channel-ID")
	resourceID := r.Header.Get("X-Goog-Resource-ID")
	channel, account, err := h.db.GetChannelAndAccountByID(channelID)
	if err != nil {
		return
	} else if channel == nil {
		h.Debug("channel not found: %s", channelID)
		return
	}

	// sanity check
	if channel.ResourceID != resourceID {
		err = fmt.Errorf("channel and request resourceIDs do not match: %s != %s",
			channel.ResourceID, resourceID)
		return
	}

	reminderSubscriptions, err := h.db.GetReminderSubscriptionsByAccountAndCalendar(
		account, channel.CalendarID, SubscriptionTypeReminder)
	if err != nil {
		return
	}
	inviteSubscriptions, err := h.db.GetReminderSubscriptionsByAccountAndCalendar(
		account, channel.CalendarID, SubscriptionTypeInvite)
	if err != nil {
		return
	}

	srv, err := GetCalendarService(account, h.oauth)
	if err != nil {
		return
	}

	registerForReminders := func(start time.Time, isAllDay bool, event *calendar.Event) {
		if isAllDay {
			// TODO(marcel): support all day event reminders
			return
		}
		// check if the event starts in the next 3 hours before registering it
		if time.Now().Before(start) && time.Now().Add(3*time.Hour).After(start) {
			for _, subscription := range reminderSubscriptions {
				err = h.reminderScheduler.UpdateOrCreateReminderEvent(account, subscription, event)
				if err != nil {
					return
				}
			}
		}
	}

	sendInvites := func(end time.Time, event *calendar.Event) {
		if event.RecurringEventId != "" && event.RecurringEventId != event.Id {
			// if the event is recurring, only deal with the underlying recurring event
			return
		}
		if time.Now().After(end) {
			// the event has already ended, don't send an invite
			return
		}
		var exists bool
		exists, err = h.db.ExistsInvite(account, channel.CalendarID, event.Id)
		if err != nil {
			return
		}
		if !exists {
			// user was recently invited to the event
			for range inviteSubscriptions {
				// TODO(marcel): use subscription convid
				err = h.handler.sendEventInvite(account, channel, event)
				if err != nil {
					return
				}
			}
		}
	}

	var events []*calendar.Event
	nextSyncToken := channel.NextSyncToken
	err = srv.Events.
		List(channel.CalendarID).
		SyncToken(channel.NextSyncToken).
		Pages(context.Background(), func(page *calendar.Events) error {
			nextSyncToken = page.NextSyncToken
			events = append(events, page.Items...)
			return nil
		})
	switch typedErr := err.(type) {
	case nil:
	case *googleapi.Error:
		if typedErr.Code == 410 {
			// TODO(marcel): next sync token has expired, need to do a "full refresh"
			// could lead to really old events not in db having invites sent out
			return
		}
	default:
		err = fmt.Errorf("error updating events for user '%s' account '%s', cal '%s': %s",
			account.KeybaseUsername, account.AccountNickname, channel.CalendarID, typedErr)
		return
	}

	for _, event := range events {
		status := EventStatus(event.Status)

		if status == EventStatusCancelled {
			for _, subscription := range reminderSubscriptions {
				err = h.reminderScheduler.UpdateOrCreateReminderEvent(account, subscription, event)
				if err != nil {
					return
				}
			}
			continue
		}

		var start, end time.Time
		var isAllDay bool
		start, end, isAllDay, err = ParseTime(event.Start, event.End)
		if err != nil {
			return
		}

		if event.Attendees == nil {
			// the event has no attendees, the user created it! register for reminders
			registerForReminders(start, isAllDay, event)
		}

		for _, attendee := range event.Attendees {
			responseStatus := ResponseStatus(attendee.ResponseStatus)
			if attendee.Self && (responseStatus == ResponseStatusAccepted || responseStatus == ResponseStatusTentative) {
				// the user has (possibly tentatively) accepted the event invite, register for reminders
				registerForReminders(start, isAllDay, event)
			} else if attendee.Self && !attendee.Organizer && responseStatus == ResponseStatusNeedsAction &&
				status != EventStatusCancelled {
				// the user has not responded to the event invite, send event invites
				sendInvites(end, event)
			}
		}
	}

	err = h.db.UpdateChannelNextSyncToken(channelID, nextSyncToken)
	if err != nil {
		return
	}

	h.Stats.Count("handleEventUpdateWebhook")
	h.Stats.CountMult("handleEventUpdateWebhook - events", len(events))

	w.WriteHeader(200)
}

func (h *Handler) createSubscription(
	account *Account, subscription Subscription,
) (exists bool, err error) {
	exists, err = h.db.ExistsSubscription(account, subscription)
	if err != nil || exists {
		// if no error, subscription exists, short circuit
		return exists, err
	}

	err = h.createEventChannel(account, subscription.CalendarID)
	if err != nil {
		return exists, err
	}

	err = h.db.InsertSubscription(account, subscription)
	if err != nil {
		return exists, err
	}

	h.reminderScheduler.AddSubscription(account, subscription)

	return false, nil
}

func (h *Handler) removeSubscription(
	account *Account, subscription Subscription,
) error {
	err := h.db.DeleteSubscription(account, subscription)
	if err != nil {
		// if no error, subscription doesn't exist, short circuit
		return err
	}

	h.reminderScheduler.RemoveSubscription(account, subscription)

	subscriptionCount, err := h.db.CountSubscriptionsByAccountAndCalender(account, subscription.CalendarID)
	if err != nil {
		return err
	}

	if subscriptionCount == 0 {
		// if there are no more subscriptions for this account + calendar, remove the channel
		channel, err := h.db.GetChannel(account, subscription.CalendarID)
		if err != nil {
			return err
		}

		if channel != nil {
			srv, err := GetCalendarService(account, h.oauth)
			if err != nil {
				return err
			}
			err = srv.Channels.Stop(&calendar.Channel{
				Id:         channel.ChannelID,
				ResourceId: channel.ResourceID,
			}).Do()
			switch err := err.(type) {
			case nil:
			case *googleapi.Error:
				if err.Code != 404 {
					return err
				}
				// if the channel wasn't found, don't return
			default:
				return err
			}

			err = h.db.DeleteChannelByChannelID(channel.ChannelID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (h *Handler) createEventChannel(account *Account, calendarID string) error {
	srv, err := GetCalendarService(account, h.oauth)
	if err != nil {
		return err
	}
	exists, err := h.db.ExistsChannelByAccountAndCalendar(account, calendarID)
	if err != nil || exists {
		// if err is nil but the channel exists, return
		return err
	}

	// channel not found, create one
	channelID, err := base.MakeRequestID()
	if err != nil {
		return err
	}

	// TODO(marcel): possibly fill in existing invites into db
	// TODO(marcel): this process can take seconds and forces the config page to load for that time
	// get all events simply to get the NextSyncToken and begin receiving invites from there
	// request only token fields so that the responses are tiny and fast
	syncStart := time.Now()
	var nextSyncToken string
	err = srv.Events.List(calendarID).
		Fields("nextPageToken", "nextSyncToken").
		Pages(context.Background(), func(page *calendar.Events) error {
			if page.NextPageToken == "" {
				// set the sync token when the page token is empty
				nextSyncToken = page.NextSyncToken
			}
			return nil
		})
	if err != nil {
		return err
	}
	h.stats.Value("createEventChannel - sync - duration - seconds", time.Since(syncStart).Seconds())

	// open channel
	res, err := srv.Events.Watch(calendarID, &calendar.Channel{
		Address: fmt.Sprintf("%s/gcalbot/events/webhook", h.httpPrefix),
		Id:      channelID,
		Type:    "web_hook",
	}).Do()
	if err != nil {
		return err
	}

	err = h.db.InsertChannel(account, Channel{
		ChannelID:     channelID,
		CalendarID:    calendarID,
		ResourceID:    res.ResourceId,
		Expiry:        time.Unix(res.Expiration/1e3, 0),
		NextSyncToken: nextSyncToken,
	})

	return err
}

type RenewChannelScheduler struct {
	*base.DebugOutput
	sync.Mutex

	shutdownCh chan struct{}

	stats      *base.StatsRegistry
	db         *DB
	config     *oauth2.Config
	httpPrefix string
}

func NewRenewChannelScheduler(
	stats *base.StatsRegistry,
	debugConfig *base.ChatDebugOutputConfig,
	db *DB,
	config *oauth2.Config,
	httpPrefix string,
) *RenewChannelScheduler {
	return &RenewChannelScheduler{
		stats:       stats.SetPrefix("RenewChannelScheduler"),
		DebugOutput: base.NewDebugOutput("RenewChannelScheduler", debugConfig),
		db:          db,
		config:      config,
		httpPrefix:  httpPrefix,
		shutdownCh:  make(chan struct{}),
	}
}

func (r *RenewChannelScheduler) Shutdown() error {
	r.Lock()
	defer r.Unlock()
	if r.shutdownCh != nil {
		close(r.shutdownCh)
		r.shutdownCh = nil
	}
	return nil
}

func (r *RenewChannelScheduler) Run() error {
	r.Lock()
	shutdownCh := r.shutdownCh
	r.Unlock()
	r.renewScheduler(shutdownCh)
	r.Debug("shut down")
	return nil
}

func (r *RenewChannelScheduler) renewScheduler(shutdownCh chan struct{}) {
	ticker := time.NewTicker(time.Hour)
	defer func() {
		ticker.Stop()
		r.Debug("shutting down")
	}()
	for {
		select {
		case <-shutdownCh:
			return
		case renewMinute := <-ticker.C:
			pairs, err := r.db.GetExpiringChannelAndAccountList()
			if err != nil {
				r.Errorf("error getting expiring pairs: %s", err)
			}
			for _, pair := range pairs {
				select {
				case <-shutdownCh:
					return
				default:
				}
				err = r.renewChannel(&pair.Account, &pair.Channel)
				if err != nil {
					r.Errorf("error renewing channel '%s': %s", pair.Channel.ChannelID, err)
				}
			}
			r.stats.Value("renewScheduler - duration - seconds", time.Since(renewMinute).Seconds())
		}
	}
}

func (r *RenewChannelScheduler) renewChannel(account *Account, channel *Channel) error {
	r.stats.Count("renewChannel")
	srv, err := GetCalendarService(account, r.config)
	if err != nil {
		return err
	}

	newChannelID, err := base.MakeRequestID()
	if err != nil {
		return err
	}

	// open new channel
	res, err := srv.Events.Watch(channel.CalendarID, &calendar.Channel{
		Address: fmt.Sprintf("%s/gcalbot/events/webhook", r.httpPrefix),
		Id:      newChannelID,
		Type:    "web_hook",
	}).Do()
	if err != nil {
		return err
	}

	err = r.db.UpdateChannel(channel.ChannelID, newChannelID, time.Unix(res.Expiration/1e3, 0))
	if err != nil {
		return err
	}

	// close old channel
	err = srv.Channels.Stop(&calendar.Channel{
		Id:         channel.ChannelID,
		ResourceId: channel.ResourceID,
	}).Do()
	switch err := err.(type) {
	case nil:
	case *googleapi.Error:
		if err.Code != 404 {
			return err
		}
		// if the channel wasn't found, don't return an error
	default:
		return err
	}

	return nil
}
