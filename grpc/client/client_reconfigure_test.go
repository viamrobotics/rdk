package client_test

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/test"
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
	defer robot.Close(ctx)

	err = robot.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	// start robot client
	robotClient, err := client.New(
		ctx1,
		addr,
		logger,
		client.WithCheckConnectedEvery(time.Second),
		client.WithReconnectEvery(time.Second),
	)
	test.That(t, err, test.ShouldBeNil)
	defer robotClient.Close(ctx1)

	res, err := arm.FromRobot(robotClient, "arm1")
	test.That(t, err, test.ShouldBeNil)
	for i := 0; i < 2; i++ {
		test.That(t, robotClient.ResourceNames(), test.ShouldNotBeNil)
		_, err := res.GetEndPosition(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		time.Sleep(time.Second)
	}

	// close/disconnect the robot
	robot.Close(ctx)
	// reconnect the robot

	// start robot
	robot, err = robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robot, test.ShouldNotBeNil)
	defer robot.Close(ctx)

	err = robot.StartWeb(ctx, options)
	test.That(t, err, test.ShouldBeNil)

	time.Sleep(time.Second * 20)
	// these should now still be found
	test.That(t, robotClient.ResourceNames(), test.ShouldNotBeNil)
	_, err = res.GetEndPosition(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(time.Second)

}
