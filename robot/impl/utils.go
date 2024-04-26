package robotimpl

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot"
)

// TODO(RSDK-7299): This function duplicates `robotimpltest.LocalRobot` for tests that
// are in the `robotimpl` package. Importing `robotimpl.LocalRobot` for those tests
// creates a circular import, and changing those tests to be in the `robotimpl_test`
// package causes failures because they test private methods.
func setupLocalRobot(
	t *testing.T,
	ctx context.Context,
	cfg *config.Config,
	logger logging.Logger,
) robot.LocalRobot {
	t.Helper()

	// use a temporary home directory so that it doesn't collide with
	// the user's/other tests' viam home directory
	r, err := New(ctx, cfg, logger, WithModHomeDir(t.TempDir()))
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() {
		test.That(t, r.Close(ctx), test.ShouldBeNil)
	})
	return r
}
