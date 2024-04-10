package robotimpl

import (
	"context"
	"testing"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot"
	"go.viam.com/test"
)

// TODO: this duplicates robotimpltest.LocalRobot
//
//revive:disable-next-line:context-as-argument
func setupLocalRobot(
	t *testing.T,
	ctx context.Context,
	cfg *config.Config,
	logger logging.Logger,
) robot.LocalRobot {
	t.Helper()

	r, err := New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r, test.ShouldNotBeNil)
	t.Cleanup(func() {
		test.That(t, r.Close(ctx), test.ShouldBeNil)
	})
	return r
}
