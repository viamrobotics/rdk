package client_test

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/resource"
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
	defer robotClient.Close(ctx1)

	res, err := arm.FromRobot(robotClient, "arm1")
	test.That(t, err, test.ShouldBeNil)
	// these should now still be found
	ch := make(chan int)
	ch1 := make(chan error)
	go checkRobot(t, robotClient, res, ch, ch1)

	test.That(t, <-ch, test.ShouldEqual, 4)
	test.That(t, <-ch1, test.ShouldBeNil)

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
	test.That(t, <-ch, test.ShouldEqual, 4)
	test.That(t, <-ch1, test.ShouldBeNil)
	close(ch)

}

func checkRobot(t *testing.T, c *client.RobotClient, a arm.Arm, ch chan int, ch1 chan error) {
	var names []resource.Name
	for {
		names = c.ResourceNames()
		ch <- len(names)
		_, err := a.GetEndPosition(context.Background(), map[string]interface{}{})
		ch1 <- err
		time.Sleep(time.Second)
	}
}
