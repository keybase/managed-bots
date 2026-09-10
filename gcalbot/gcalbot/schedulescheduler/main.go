package schedulescheduler

import (
	"context"
	"sync"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/managed-bots/base"
	"github.com/keybase/managed-bots/gcalbot/gcalbot"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
)

type ScheduleScheduler struct {
	*base.DebugOutput
	sync.Mutex

	shutdownCh chan struct{}

	stats *base.StatsRegistry
	db    *gcalbot.DB
	oauth *oauth2.Config
	kbc   *kbchat.API
}

func NewScheduleScheduler(
	stats *base.StatsRegistry,
	debugConfig *base.ChatDebugOutputConfig,
	db *gcalbot.DB,
	oauth *oauth2.Config,
	kbc *kbchat.API,
) *ScheduleScheduler {
	return &ScheduleScheduler{
		stats:       stats.SetPrefix("ScheduleScheduler"),
		DebugOutput: base.NewDebugOutput("ScheduleScheduler", debugConfig),
		shutdownCh:  make(chan struct{}),
		db:          db,
		oauth:       oauth,
		kbc:         kbc,
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

func (s *ScheduleScheduler) getCalendarService(ctx context.Context, account *gcalbot.Account) (*calendar.Service, error) {
	return gcalbot.GetCalendarServiceWithRetry(ctx, account, s.oauth, s.db, s.DebugOutput, s.kbc)
}

func (s *ScheduleScheduler) wrapAuth(ctx context.Context, account *gcalbot.Account, err error) error {
	return gcalbot.WrapAuthError(ctx, account, err, s.db, s.DebugOutput, s.kbc)
}
