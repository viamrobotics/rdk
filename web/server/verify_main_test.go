// Package server implements the entry point for running a robot web server.
package server

import (
	"testing"

	"go.uber.org/goleak"
	testutilsext "go.viam.com/utils/testutils/ext"
)

// TestMain is used to control the execution of all tests run within this package (including _test packages).
func TestMain(m *testing.M) {
	testutilsext.VerifyTestMain(m, testutilsext.WithLeakOpt(goleak.IgnoreTopFunction("gopkg.in/natefinch/lumberjack%2ev2.(*Logger).millRun")))
}
