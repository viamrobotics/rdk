package utils

import "go.uber.org/goleak"

// FindGoroutineLeaks finds any goroutine leaks after a program is done running. This
// should be used at the end of a main test run or a top-level process run.
func FindGoroutineLeaks() error {
	return goleak.Find(
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("github.com/desertbit/timer.timerRoutine"), // gRPC uses this
	)
}
