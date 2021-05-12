package testutils

import (
	"go.uber.org/goleak"
)

// VerifyTestMain preforms various runtime checks on code that tests run.
func VerifyTestMain(m goleak.TestingM) {
	// Verify no goroutine leaks occur by the end of the test.
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("github.com/desertbit/timer.timerRoutine"), // gRPC uses this
	)
}
