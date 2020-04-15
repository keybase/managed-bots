package reminderscheduler

import (
	"sync"

	"github.com/keybase/managed-bots/base"
	"github.com/keybase/managed-bots/gcalbot/gcalbot"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
)

type ReminderScheduler struct {
	*base.DebugOutput
	sync.Mutex

	shutdownCh chan struct{}

	stats *base.StatsRegistry
	db    *gcalbot.DB
	oauth *oauth2.Config

	subscriptionReminders *SubscriptionReminders
	eventReminders        *EventReminders
	minuteReminders       *MinuteReminders
}

func NewReminderScheduler(
	stats *base.StatsRegistry,
	debugConfig *base.ChatDebugOutputConfig,
	db *gcalbot.DB,
	oauth *oauth2.Config,
) *ReminderScheduler {
	return &ReminderScheduler{
		stats:                 stats.SetPrefix("ReminderScheduler"),
		DebugOutput:           base.NewDebugOutput("ReminderScheduler", debugConfig),
		shutdownCh:            make(chan struct{}),
		db:                    db,
		oauth:                 oauth,
		subscriptionReminders: NewSubscriptionReminders(),
		eventReminders:        NewEventReminders(),
		minuteReminders:       NewMinuteReminders(),
	}
}

func (r *ReminderScheduler) Run() (err error) {
	defer r.Trace(&err, "Run")()
	r.Lock()
	shutdownCh := r.shutdownCh
	r.Unlock()
	eg := &errgroup.Group{}
	base.GoWithRecoverErrGroup(eg, r.DebugOutput, func() error { return r.eventSyncLoop(shutdownCh) })
	base.GoWithRecoverErrGroup(eg, r.DebugOutput, func() error { return r.sendReminderLoop(shutdownCh) })
	if err := eg.Wait(); err != nil {
		r.Debug("wait error: %s", err)
		return err
	}
	return nil
}

func (r *ReminderScheduler) Shutdown() (err error) {
	defer r.Trace(&err, "Shutdown")()
	r.Lock()
	defer r.Unlock()
	if r.shutdownCh != nil {
		close(r.shutdownCh)
		r.shutdownCh = nil
	}
	return nil
}
