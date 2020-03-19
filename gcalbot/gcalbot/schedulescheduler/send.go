package schedulescheduler

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"golang.org/x/oauth2"

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

	s.sendDailySchedulesForMinute(time.Now(), shutdownCh)
	for {
		select {
		case <-shutdownCh:
			return nil
		case sendMinute := <-ticker.C:
			s.sendDailySchedulesForMinute(sendMinute, shutdownCh)
		}
	}
}

func (s *ScheduleScheduler) sendDailySchedulesForMinute(sendMinute time.Time, shutdownCh chan struct{}) {
	if sendMinute.Minute()%30 != 0 {
		s.Errorf("daily schedule loop out of sync, sendMinute: %s", sendMinute.Format("15:04:05"))
	}

	var subscriptions []*gcalbot.AggregatedDailyScheduleSubscription

	todaySubscriptions, err := s.db.GetAggregatedDailyScheduleSubscription(gcalbot.ScheduleToSendToday)
	if err != nil {
		s.Errorf("error getting daily schedule subscriptions to sync: %s", err)
	}
	subscriptions = todaySubscriptions

	tomorrowSubscriptions, err := s.db.GetAggregatedDailyScheduleSubscription(gcalbot.ScheduleToSendTomorrow)
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
	userSendMinute := sendMinute.In(subscription.Timezone)
	minutesSinceMidnight := float64(userSendMinute.Hour()*60 + userSendMinute.Minute())

	// if the duration minutes are more than 5 minutes apart, return
	if math.Abs(minutesSinceMidnight-subscription.NotificationTime.Minutes()) > 5 {
		return
	}

	switch subscription.DaysToSend {
	case gcalbot.DaysToSendEveryday:
	case gcalbot.DaysToSendMonToFri:
		switch userSendMinute.Weekday() {
		case time.Saturday, time.Sunday:
			return
		}
	case gcalbot.DaysToSendSunToThu:
		switch userSendMinute.Weekday() {
		case time.Friday, time.Saturday:
			return
		}
	}

	s.stats.Count("SendDailyScheduleMessage")
	s.stats.CountMult("SendDailyScheduleMessage - calendars", len(subscription.CalendarIDs))

	srv, err := gcalbot.GetCalendarService(&subscription.Account, s.oauth, s.db)
	switch err.(type) {
	case nil:
	case *oauth2.RetrieveError:
		s.Debug("error retrieving token: %s", err)
		return
	default:
		s.Errorf("unable to get calendar service: %s", err)
		return
	}

	var minTime time.Time
	switch subscription.ScheduleToSend {
	case gcalbot.ScheduleToSendToday:
		minTime = time.Date(userSendMinute.Year(), userSendMinute.Month(), userSendMinute.Day(), 0, 0, 0, 0, subscription.Timezone)
	case gcalbot.ScheduleToSendTomorrow:
		minTime = time.Date(userSendMinute.Year(), userSendMinute.Month(), userSendMinute.Day()+1, 0, 0, 0, 0, subscription.Timezone)
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

	message := `Here is what's happening %s, *%s*
Account: %s
Calendars: %s
%s
%s`

	date := minTime.Format("Monday, January 2")

	calendarList := strings.Join(calendarSummaries, ", ")

	link := fmt.Sprintf("https://calendar.google.com/calendar/r/day/%d/%d/%d",
		userSendMinute.Year(), userSendMinute.Month(), userSendMinute.Day())

	var formattedSchedule string
	if len(events) == 0 {
		formattedSchedule = "> You have no events today :sunny:"
	} else {
		formattedSchedule, err = gcalbot.FormatEventSchedule(events, subscription.Timezone, format24HourTime)
		if err != nil {
			s.Errorf("unable to format schedule: %s", err)
			return
		}
	}

	s.ChatEcho(subscription.KeybaseConvID, message,
		subscription.ScheduleToSend, date, subscription.Account.AccountNickname, calendarList,
		formattedSchedule, link)
}
