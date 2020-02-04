package base

import (
	"time"

	"runtime/debug"

	"os"

	"golang.org/x/sync/errgroup"
)

func PanicRecover(debugOutput *DebugOutput) {
	if r := recover(); r != nil {
		debugOutput.Errorf("panic: %v stack trace: %s", r, debug.Stack())
		time.Sleep(2 * time.Second) // sleep so that we can get this message to logging infrastructure
		os.Exit(1)
	}
}

func GoWithRecoverErrGroup(eg *errgroup.Group, debugOutput *DebugOutput, f func() error) {
	eg.Go(func() error {
		defer PanicRecover(debugOutput)
		return f()
	})
}

func GoWithRecover(debugOutput *DebugOutput, f func()) {
	go func() {
		defer PanicRecover(debugOutput)
		f()
	}()
}
