package robotimpl

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/testutils"
)

func setupLocalRobot(
	t *testing.T,
	ctx context.Context,
	cfg *config.Config,
	logger logging.Logger,
) robot.LocalRobot {
	t.Helper()

	// use a temporary home directory so that it doesn't collide with
	// the user's/other tests' viam home directory
	r, err := New(ctx, cfg, logger, WithViamHomeDir(t.TempDir()))
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() {
		test.That(t, r.Close(ctx), test.ShouldBeNil)
		// Wait for reconfigureWorkers here because localRobot.Close does not.
		lRobot, ok := r.(*localRobot)
		test.That(t, ok, test.ShouldBeTrue)
		lRobot.reconfigureWorkers.Wait()
	})
	return r
}

func verifyReadyResourceNames(tb testing.TB, r robot.LocalRobot, expected []resource.Name) {
	tb.Helper()

	ms, err := r.MachineStatus(context.Background())
	test.That(tb, err, test.ShouldBeNil)

	var ready []resource.Name
	for _, rs := range ms.Resources {
		if rs.State == resource.NodeStateReady {
			ready = append(ready, rs.Name)
		}
	}
	testutils.VerifySameResourceNames(tb, ready, expected)
}
