package schedulescheduler

import (
	"sync"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
	"github.com/keybase/managed-bots/gcalbot/gcalbot"
	"golang.org/x/oauth2"
)

type ScheduleScheduler struct {
	*base.DebugOutput
	sync.Mutex

	shutdownCh chan struct{}

	stats *base.StatsRegistry
	db    *gcalbot.DB
	cal   *gcalbot.CalendarAuth
}

func NewScheduleScheduler(
	stats *base.StatsRegistry,
	debugConfig *base.ChatDebugOutputConfig,
	db *gcalbot.DB,
	oauth *oauth2.Config,
	kbc *kbchat.API,
) *ScheduleScheduler {
	debug := base.NewDebugOutput("ScheduleScheduler", debugConfig)
	return &ScheduleScheduler{
		stats:       stats.SetPrefix("ScheduleScheduler"),
		DebugOutput: debug,
		shutdownCh:  make(chan struct{}),
		db:          db,
		cal:         gcalbot.NewCalendarAuth(oauth, db, debug, kbc),
	}
}

func (s *ScheduleScheduler) Run() (err error) {
	defer s.Trace(&err, "Run")()
	s.Lock()
	shutdownCh := s.shutdownCh
	s.Unlock()
	if err = s.sendDailyScheduleLoop(shutdownCh); err != nil {
		return err
	}
	return nil
}

func (s *ScheduleScheduler) Shutdown() (err error) {
	defer s.Trace(&err, "Shutdown")()
	s.Lock()
	defer s.Unlock()
	if s.shutdownCh != nil {
		close(s.shutdownCh)
		s.shutdownCh = nil
	}
	return nil
}
