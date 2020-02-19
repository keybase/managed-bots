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

func (s *ScheduleScheduler) Run() error {
	s.Lock()
	shutdownCh := s.shutdownCh
	s.Unlock()
	err := s.sendDailyScheduleLoop(shutdownCh)
	if err != nil {
		return err
	}
	s.Debug("shut down")
	return nil
}

func (s *ScheduleScheduler) Shutdown() error {
	s.Lock()
	defer s.Unlock()
	if s.shutdownCh != nil {
		close(s.shutdownCh)
		s.shutdownCh = nil
	}
	return nil
}
