// Package testutilsext is purely for test utilities that may access other packages
// in the codebase that tend to use testutils.
package testutilsext

import (
	"fmt"
	"os"

	"go.uber.org/goleak"

	"go.viam.com/core/artifact"
	"go.viam.com/core/rlog"
)

// VerifyTestMain preforms various runtime checks on code that tests run.
func VerifyTestMain(m goleak.TestingM) {
	cache, err := artifact.GlobalCache()
	if err != nil {
		rlog.Logger.Fatal("error opening artifact", "error", err)
	}
	exitCode := m.Run()
	if err := cache.Close(); err != nil {
		rlog.Logger.Errorw("error closing artifact", "error", err)
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	if err := goleak.Find(
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("github.com/desertbit/timer.timerRoutine"), // gRPC uses this
	); err != nil {
		fmt.Fprintf(os.Stderr, "goleak: Errors on successful test run: %v\n", err)
		os.Exit(1)
	}
}
