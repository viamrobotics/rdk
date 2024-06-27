// Package robotimpltest provides testing utilities related to the robotimpl package.
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

// SetupLocalRobot create a local robot from a given config and ensure it is shutdown
// after the test is complete. This includes waiting for any reconfiguration workers to
// finish.
//
// Duplicate of robot/impl/utils.go#setupLocalRobot.
func SetupLocalRobot(
	t *testing.T,
	ctx context.Context,
	cfg *config.Config,
	logger logging.Logger,
) robot.LocalRobot {
	t.Helper()

	// use a temporary home directory so that it doesn't collide with
	// the user's/other tests' viam home directory
	r, err := robotimpl.New(ctx, cfg, logger, robotimpl.WithViamHomeDir(t.TempDir()))
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() {
		test.That(t, r.CloseWait(ctx), test.ShouldBeNil)
	})
	return r
}
