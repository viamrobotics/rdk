package utils

import "go.uber.org/goleak"

// FindGoroutineLeaks finds any goroutine leaks after a program is done running. This
// should be used at the end of a main test run or a top-level process run.
func FindGoroutineLeaks(options ...goleak.Option) error {
	optsCopy := make([]goleak.Option, len(options))
	copy(optsCopy, options)
	optsCopy = append(optsCopy,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("github.com/desertbit/timer.timerRoutine"),              // gRPC uses this
		goleak.IgnoreTopFunction("github.com/letsencrypt/pebble/va.VAImpl.processTasks"), // no way to stop it,
	)
	return goleak.Find(optsCopy...)
}
