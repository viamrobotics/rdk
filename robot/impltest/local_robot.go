// Package robotimpltest contains utilities for testing robotimpl functionality
package robotimpltest

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
)

// LocalRobot returns a new robot with parts sourced from the given config, or fails the
// test if it cannot. It automatically closes itself after the test and all subtests
// complete.
func LocalRobot(
	t *testing.T,
	ctx context.Context,
	cfg *config.Config,
	logger logging.Logger,
) robot.LocalRobot {
	t.Helper()

	// use a temporary home directory so that it doesn't collide with
	// the user's/other tests' viam home directory
	r, err := robotimpl.New(ctx, cfg, logger, robotimpl.WithModHomeDir(t.TempDir()))
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() {
		test.That(t, r.Close(ctx), test.ShouldBeNil)
	})
	return r
}
