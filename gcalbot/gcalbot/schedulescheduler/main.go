package schedulescheduler

import (
	"sync"

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
	oauth *oauth2.Config
}

func NewScheduleScheduler(
	stats *base.StatsRegistry,
	debugConfig *base.ChatDebugOutputConfig,
	db *gcalbot.DB,
	oauth *oauth2.Config,
) *ScheduleScheduler {
	return &ScheduleScheduler{
		stats:       stats.SetPrefix("ScheduleScheduler"),
		DebugOutput: base.NewDebugOutput("ScheduleScheduler", debugConfig),
		shutdownCh:  make(chan struct{}),
		db:          db,
		oauth:       oauth,
	}
}

func (s *ScheduleScheduler) Run() (err error) {
	defer s.Trace(func() error { return err }, "Run")()
	s.Lock()
	shutdownCh := s.shutdownCh
	s.Unlock()
	if err = s.sendDailyScheduleLoop(shutdownCh); err != nil {
		return err
	}
	return nil
}

func (s *ScheduleScheduler) Shutdown() (err error) {
	defer s.Trace(func() error { return err }, "Shutdown")()
	s.Lock()
	defer s.Unlock()
	if s.shutdownCh != nil {
		close(s.shutdownCh)
		s.shutdownCh = nil
	}
	return nil
}
