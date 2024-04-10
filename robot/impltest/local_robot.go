package robotimpltest

import (
	"context"
	"testing"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/test"
)

// LocalRobot returns a new robot with parts sourced from the given config, or fails the
// test if it cannot. It automatically closes itself after the test and all subtests
// complete.
//
//revive:disable-next-line:context-as-argument
func LocalRobot(
	t *testing.T,
	ctx context.Context,
	cfg *config.Config,
	logger logging.Logger,
) robot.LocalRobot {
	t.Helper()

	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r, test.ShouldNotBeNil)
	t.Cleanup(func() {
		test.That(t, r.Close(ctx), test.ShouldBeNil)
	})
	return r
}
