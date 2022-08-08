package client_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.uber.org/zap"
	"go.viam.com/rdk/robot"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/component/arm"
	_ "go.viam.com/rdk/component/arm/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc/client"
	robotimpl "go.viam.com/rdk/robot/impl"
)

func startBaseRobot(t *testing.T, logger *zap.SugaredLogger, ctx context.Context, listener net.Listener, cfgFile string) (robot.LocalRobot, weboptions.Options) {
	cfg, err := config.Read(ctx, cfgFile, logger)
	test.That(t, err, test.ShouldBeNil)

	options := weboptions.New()
	options.Network.BindAddress = ""

	options.Network.Listener = listener

	// start robot
	robot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robot, test.ShouldNotBeNil)
	return robot, options
}

func newRobotClient(t *testing.T, logger *zap.SugaredLogger, addr string) *client.RobotClient {
	dur := time.Second
	// start robot client
	robotClient, err := client.New(
		context.Background(),
		addr,
		logger,
		client.WithCheckConnectedEvery(dur),
		client.WithReconnectEvery(dur),
	)
	test.That(t, err, test.ShouldBeNil)
	return robotClient
}

func TestReconfigurableClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	var listener net.Listener = testutils.ReserveRandomListener(t)
	robot, options := startBaseRobot(t, logger, ctx, listener, "data/robot0.json")
	addr := listener.Addr().String()
	defer func() {
		test.That(t, utils.TryClose(context.Background(), robot), test.ShouldBeNil)
	}()
	err := robot.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// start robot client
	robotClient := newRobotClient(t, logger, addr)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), robotClient), test.ShouldBeNil)
	}()

	test.That(t, len(robotClient.ResourceNames()), test.ShouldEqual, 4)
	res, err := arm.FromRobot(robotClient, "arm1")
	test.That(t, err, test.ShouldBeNil)
	_, err = res.GetEndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

	// close/disconnect the robot
	test.That(t, utils.TryClose(context.Background(), robot), test.ShouldBeNil)
	test.That(t, <-robotClient.Changed(), test.ShouldBeTrue)
	test.That(t, len(robotClient.ResourceNames()), test.ShouldEqual, 0)
	_, err = robotClient.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeError)

	// reconnect the robot
	ctx2 := context.Background()
	listener, err = net.Listen("tcp", listener.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	robot, options = startBaseRobot(t, logger, ctx2, listener, "data/robot0.json")

	err = robot.StartWeb(ctx2, options)
	test.That(t, err, test.ShouldBeNil)

	// check if the original arm can still be called
	test.That(t, <-robotClient.Changed(), test.ShouldBeTrue)
	test.That(t, robotClient.Connected(), test.ShouldBeTrue)
	test.That(t, len(robotClient.ResourceNames()), test.ShouldEqual, 4)
	_, err = robotClient.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = res.GetEndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
}

func TestReconnectRemote(t *testing.T) {
	logger := golog.NewTestLogger(t)

	// start the first robot
	ctx := context.Background()
	var listener net.Listener = testutils.ReserveRandomListener(t)
	robot, options := startBaseRobot(t, logger, ctx, listener, "data/robot0.json")
	// addr := listener.Addr().String()
	defer func() {
		test.That(t, utils.TryClose(context.Background(), robot), test.ShouldBeNil)
	}()
	err := robot.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// start the second robot
	ctx1 := context.Background()
	var listener1 net.Listener = testutils.ReserveRandomListener(t)
	robot1, options1 := startBaseRobot(t, logger, ctx, listener1, "data/robot1.json")
	addr1 := listener.Addr().String()
	defer func() {
		test.That(t, utils.TryClose(context.Background(), robot1), test.ShouldBeNil)
	}()
	err = robot1.StartWeb(ctx1, options1)
	test.That(t, err, test.ShouldBeNil)

	// start the robot client that uses the first robot as a remote
	robotClient := newRobotClient(t, logger, addr1)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), robotClient), test.ShouldBeNil)
	}()

	a, err := robotClient.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, a, test.ShouldNotBeNil)
	anArm, ok := a.(arm.Arm)
	test.That(t, ok, test.ShouldBeTrue)
	_, err = anArm.GetEndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

	// close/disconnect the robot
	test.That(t, utils.TryClose(context.Background(), robot), test.ShouldBeNil)
	test.That(t, <-robotClient.Changed(), test.ShouldBeTrue)
	test.That(t, len(robotClient.ResourceNames()), test.ShouldEqual, 0)
	_, err = robotClient.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeError)

	// reconnect the first robot
	ctx2 := context.Background()
	listener, err = net.Listen("tcp", listener.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	robot, options = startBaseRobot(t, logger, ctx2, listener, "data/robot0.json")

	err = robot.StartWeb(ctx2, options)
	test.That(t, err, test.ShouldBeNil)

	// check if the original arm can still be called
	test.That(t, <-robotClient.Changed(), test.ShouldBeTrue)
	test.That(t, robotClient.Connected(), test.ShouldBeTrue)
	test.That(t, len(robotClient.ResourceNames()), test.ShouldEqual, 4)
	_, err = robotClient.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = anArm.GetEndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

}
