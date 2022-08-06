package client_test

import (
	"context"
	"fmt"
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

func startBaseRobot(t *testing.T, logger *zap.SugaredLogger, ctx context.Context) (robot.LocalRobot, net.Listener) {
	cfg, err := config.Read(ctx, "data/robot0.json", logger)
	test.That(t, err, test.ShouldBeNil)

	options := weboptions.New()
	options.Network.BindAddress = ""
	var listener net.Listener = testutils.ReserveRandomListener(t)
	options.Network.Listener = listener

	// start robot
	robot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robot, test.ShouldNotBeNil)
	return robot, listener
}

func TestReconfigurableClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	ctx1 := context.Background()
	// startBaseRobot(t, logger, ctx)
	cfg, err := config.Read(ctx, "data/robot0.json", logger)
	test.That(t, err, test.ShouldBeNil)

	options := weboptions.New()
	options.Network.BindAddress = ""
	var listener net.Listener = testutils.ReserveRandomListener(t)
	addr := listener.Addr().String()
	options.Network.Listener = listener
	cfg.Network.BindAddress = addr

	// start robot
	robot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robot, test.ShouldNotBeNil)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), robot), test.ShouldBeNil)
	}()
	err = robot.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	dur := time.Second
	// start robot client
	robotClient, err := client.New(
		ctx1,
		addr,
		logger,
		client.WithCheckConnectedEvery(dur),
		client.WithReconnectEvery(dur),
	)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), robotClient), test.ShouldBeNil)
	}()

	fmt.Println(robotClient.ResourceNames())
	test.That(t, len(robotClient.ResourceNames()), test.ShouldEqual, 4)
	res, err := arm.FromRobot(robotClient, "arm1")
	test.That(t, err, test.ShouldBeNil)
	_, err = res.GetEndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)

	// close/disconnect the robot
	// err = robot.StopWeb()
	test.That(t, utils.TryClose(context.Background(), robot), test.ShouldBeNil)
	// test.That(t, err, test.ShouldBeNil)
	test.That(t, <-robotClient.Changed(), test.ShouldBeTrue)
	test.That(t, len(robotClient.ResourceNames()), test.ShouldEqual, 0)
	_, err = robotClient.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeError)

	// reconnect the robot
	ctx2 := context.Background()
	robot, err = robotimpl.New(ctx2, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robot, test.ShouldNotBeNil)

	// listener reestablished
	listener, err = net.Listen("tcp", listener.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	options.Network.Listener = listener

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
	ctx := context.Background()
	cfg, err := config.Read(ctx, "data/robot0.json", logger)
	test.That(t, err, test.ShouldBeNil)

	options := weboptions.New()
	options.Network.BindAddress = ""
	listener := testutils.ReserveRandomListener(t)
	addr := listener.Addr().String()
	options.Network.Listener = listener
	cfg.Network.BindAddress = addr
	// gServer := grpc.NewServer()

	// start robot
	robot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robot, test.ShouldNotBeNil)
	defer func() {
		test.That(t, utils.TryClose(context.Background(), robot), test.ShouldBeNil)
	}()

	err = robot.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

}
