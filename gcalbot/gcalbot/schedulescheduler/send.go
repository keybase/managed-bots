package schedulescheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"

	"github.com/keybase/managed-bots/gcalbot/gcalbot"
)

func (s *ScheduleScheduler) sendDailyScheduleLoop(shutdownCh chan struct{}) error {
	// sleep until the next half hour so that the loop executes at the beginning of each half hour
	now := time.Now()
	nextHalfHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 30*(1+now.Minute()/30), 0, 0, time.Local)

	select {
	case <-shutdownCh:
		return nil
	case <-time.After(nextHalfHour.Sub(now)):
	}

	ticker := time.NewTicker(30 * time.Minute)
	defer func() {
		ticker.Stop()
		s.Debug("shutting down sendDailyScheduleLoop")
	}()

	s.sendDailySchedulesForMinute(time.Now().UTC(), shutdownCh)
	for {
		select {
		case <-shutdownCh:
			return nil
		case sendMinute := <-ticker.C:
			s.sendDailySchedulesForMinute(sendMinute.UTC(), shutdownCh)
		}
	}
}

func (s *ScheduleScheduler) sendDailySchedulesForMinute(sendMinute time.Time, shutdownCh chan struct{}) {
	dayStart := time.Date(sendMinute.Year(), sendMinute.Month(), sendMinute.Day(), 0, 0, 0, 0, time.UTC)
	notificationDuration := sendMinute.Sub(dayStart).Round(time.Minute)

	var subscriptions []*gcalbot.AggregatedDailyScheduleSubscription

	todaySubscriptions, err := s.db.GetAggregatedDailyScheduleSubscription(gcalbot.ScheduleToSendToday, notificationDuration)
	if err != nil {
		s.Errorf("error getting daily schedule subscriptions to sync: %s", err)
	}
	subscriptions = todaySubscriptions

	tomorrowSubscriptions, err := s.db.GetAggregatedDailyScheduleSubscription(gcalbot.ScheduleToSendTomorrow, notificationDuration)
	if err != nil {
		s.Errorf("error getting daily schedule subscriptions to sync: %s", err)
	}
	subscriptions = append(subscriptions, tomorrowSubscriptions...)

	for _, subscription := range subscriptions {
		select {
		case <-shutdownCh:
			return
		default:
		}

		s.SendDailyScheduleMessage(sendMinute, subscription)
	}

	s.stats.Value("sendDailySchedulesForMinute - duration - seconds", time.Since(sendMinute).Seconds())
}

func (s *ScheduleScheduler) SendDailyScheduleMessage(sendMinute time.Time, subscription *gcalbot.AggregatedDailyScheduleSubscription) {
	srv, err := gcalbot.GetCalendarService(&subscription.Account, s.oauth)
	if err != nil {
		s.Errorf(err.Error())
		return
	}

	timezone, err := gcalbot.GetUserTimezone(srv)
	if err != nil {
		s.Errorf("unable to get user timezone: %s", err)
		return
	}

	// TODO(marcel): respect users set date
	userSendMinute := sendMinute.In(timezone)

	switch subscription.DaysToSend {
	case gcalbot.DaysToSendEveryday:
	case gcalbot.DaysToSendMonToFri:
		switch userSendMinute.Weekday() {
		case time.Saturday, time.Sunday:
			return
		}
	case gcalbot.DaysToSendSatToThu:
		switch userSendMinute.Weekday() {
		case time.Friday, time.Saturday:
			return
		}
	}

	var minTime time.Time
	switch subscription.ScheduleToSend {
	case gcalbot.ScheduleToSendToday:
		minTime = time.Date(userSendMinute.Year(), userSendMinute.Month(), userSendMinute.Day(), 0, 0, 0, 0, timezone)
	case gcalbot.ScheduleToSendTomorrow:
		minTime = time.Date(userSendMinute.Year(), userSendMinute.Month(), userSendMinute.Day()+1, 0, 0, 0, 0, timezone)
	}
	maxTime := minTime.Add(24 * time.Hour)

	format24HourTime, err := gcalbot.GetUserFormat24HourTime(srv)
	if err != nil {
		s.Errorf("unable to get user 24 hour time setting: %s", err)
		return
	}

	calendarSummaries := make([]string, len(subscription.CalendarIDs))
	var events []*calendar.Event
	for index, calendarID := range subscription.CalendarIDs {
		cal, err := srv.Calendars.Get(calendarID).Fields("summary").Do()
		if err != nil {
			s.Errorf("error getting calendar summary from API: %s", err)
			calendarSummaries[index] = calendarID // use the cal id if there is an error
		} else {
			calendarSummaries[index] = cal.Summary
		}
		err = srv.Events.
			List(calendarID).
			TimeMin(minTime.Format(time.RFC3339)).
			TimeMax(maxTime.Format(time.RFC3339)).
			SingleEvents(true).
			OrderBy("startTime").
			Pages(context.Background(), func(page *calendar.Events) error {
				events = append(events, page.Items...)
				return nil
			})
		if err != nil {
			s.Errorf("error getting events from API: %s", err)
			continue
		}
	}

	// TODO(marcel): better calendar listing
	calendarList := strings.Join(calendarSummaries, ", ")
	if len(events) == 0 {
		s.ChatEcho(subscription.KeybaseConvID, "You have no events for the calendar(s) %s in account '%s' for %s.",
			calendarList, subscription.Account.AccountNickname, subscription.ScheduleToSend)
	} else {
		formattedSchedule, err := gcalbot.FormatEventSchedule(events, timezone, format24HourTime)
		if err != nil {
			s.Errorf("unable to format schedule: %s", err)
			return
		}
		link := fmt.Sprintf("https://calendar.google.com/calendar/r/day/%d/%d/%d", userSendMinute.Year(), userSendMinute.Month(), userSendMinute.Day())
		message := `%s for account '%s' - %s
Calendars: %s
%s
%s`
		s.ChatEcho(subscription.KeybaseConvID, message,
			strings.Title(string(subscription.ScheduleToSend)), subscription.Account.AccountNickname,
			userSendMinute.Format("Monday, January 2"), calendarList, link, formattedSchedule)
	}
}
