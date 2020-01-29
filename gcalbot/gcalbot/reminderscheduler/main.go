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

	db     *gcalbot.DB
	config *oauth2.Config

	eventMap         ReminderEventMap
	reminderSchedule ReminderSchedule
}

func NewReminderScheduler(
	debugConfig *base.ChatDebugOutputConfig,
	db *gcalbot.DB,
	config *oauth2.Config,
) *ReminderScheduler {
	return &ReminderScheduler{
		DebugOutput: base.NewDebugOutput("ReminderScheduler", debugConfig),
		shutdownCh:  make(chan struct{}),
		db:          db,
		config:      config,
	}
}

func (r *ReminderScheduler) Run() error {
	r.Lock()
	shutdownCh := r.shutdownCh
	r.Unlock()
	var eg errgroup.Group
	eg.Go(func() error { return r.eventSyncLoop(shutdownCh) })
	eg.Go(func() error { return r.sendReminderLoop(shutdownCh) })
	if err := eg.Wait(); err != nil {
		r.Debug("wait error: %s", err)
		return err
	}
	r.Debug("shut down")
	return nil
}

func (r *ReminderScheduler) Shutdown() error {
	r.Lock()
	defer r.Unlock()
	if r.shutdownCh != nil {
		close(r.shutdownCh)
		r.shutdownCh = nil
	}
	return nil
}
