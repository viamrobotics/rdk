package testutils

import (
	"go.uber.org/goleak"
)

func VerifyTestMain(m goleak.TestingM) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("github.com/desertbit/timer.timerRoutine"), // grpc uses this
	)
}
