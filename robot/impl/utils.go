package robotimpl

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc"
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
	opts ...Option,
) robot.LocalRobot {
	t.Helper()

	var conn rpc.ClientConn
	var err error
	if cfg.Cloud != nil && cfg.Cloud.AppAddress != "" {
		conn, err = grpc.NewAppConn(ctx, cfg.Cloud.AppAddress, cfg.Cloud.Secret, cfg.Cloud.ID, logger.Sublogger("appconn"))
		test.That(t, err, test.ShouldBeNil)
	}

	// use a temporary home directory so that it doesn't collide with
	// the user's/other tests' viam home directory
	var rOpts []Option
	rOpts = append(rOpts, opts...)
	rOpts = append(rOpts, WithViamHomeDir(t.TempDir()))
	r, err := New(ctx, cfg, conn, logger.Sublogger("rdk"), rOpts...)
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() {
		test.That(t, r.Close(ctx), test.ShouldBeNil)
		// Wait for reconfigureWorkers here because localRobot.Close does not.
		lRobot, ok := r.(*localRobot)
		test.That(t, ok, test.ShouldBeTrue)
		lRobot.reconfigureWorkers.Wait()
		if conn != nil {
			test.That(t, conn.Close(), test.ShouldBeNil)
		}
	})
	return r
}

func verifyReachableResourceNames(tb testing.TB, r robot.LocalRobot, expected []resource.Name) {
	tb.Helper()

	lRobot, ok := r.(*localRobot)
	test.That(tb, ok, test.ShouldBeTrue)

	reachable := lRobot.manager.reachableResourceNames()
	testutils.VerifySameResourceNames(tb, reachable, expected)
}
