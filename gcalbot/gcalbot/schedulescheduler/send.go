package schedulescheduler

import (
	"time"
)

func (s *ScheduleScheduler) sendDailyScheduleLoop(shutdownCh chan struct{}) error {
	// sleep until the next minute so that the loop executes at the beginning of each minute
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

	s.sendDailySchedule(time.Now())
	for {
		select {
		case <-shutdownCh:
			return nil
		case sendMinute := <-ticker.C:
			s.sendDailySchedule(sendMinute)
		}
	}
}

func (s *ScheduleScheduler) sendDailySchedule(sendMinute time.Time) {
	sendDuration := time.Since(sendMinute)
	s.stats.Value("sendDailySchedule - duration - seconds", sendDuration.Seconds())
}
