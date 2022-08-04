package client_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/edaniels/golog"
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

func TestReconfigurableClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	ctx1 := context.Background()
	cfg, err := config.Read(ctx, "data/robot0.json", logger)
	test.That(t, err, test.ShouldBeNil)

	options := weboptions.New()
	options.Network.BindAddress = ""
	listener := testutils.ReserveRandomListener(t)
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

	dur := 100 * time.Millisecond
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
	test.That(t, err, test.ShouldBeNil)
	test.That(t, <-robotClient.Changed(), test.ShouldBeTrue)
	test.That(t, len(robotClient.ResourceNames()), test.ShouldEqual, 0)
	_, err = robotClient.ResourceByName(arm.Named("arm1"))
	test.That(t, err, test.ShouldBeError)

	// reconnect the robot
	robot, err = robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robot, test.ShouldNotBeNil)

	ctx2 := context.Background()
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
