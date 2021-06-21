// Package testutilsext is purely for test utilities that may access other packages
// in the codebase that tend to use testutils.
package testutilsext

import (
	"fmt"
	"os"

	"go.uber.org/goleak"

	"go.viam.com/core/artifact"
	"go.viam.com/core/rlog"
	"go.viam.com/core/testutils"
	"go.viam.com/core/utils"
)

// VerifyTestMain preforms various runtime checks on code that tests run.
func VerifyTestMain(m goleak.TestingM) {
	cache, err := artifact.GlobalCache()
	if err != nil {
		rlog.Logger.Fatalw("error opening artifact", "error", err)
	}
	exitCode := m.Run()
	testutils.Teardown()
	if err := cache.Close(); err != nil {
		rlog.Logger.Errorw("error closing artifact", "error", err)
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	if err := utils.FindGoroutineLeaks(); err != nil {
		fmt.Fprintf(os.Stderr, "goleak: Errors on successful test run: %v\n", err)
		os.Exit(1)
	}
}
