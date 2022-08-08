package robottestutils

import (
	"context"
	"net"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.viam.com/rdk/robot"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/test"

	_ "go.viam.com/rdk/component/arm/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc/client"
	robotimpl "go.viam.com/rdk/robot/impl"
)

// StartBaseRobot creates a new local robot with a listener attached
func StartBaseRobot(t *testing.T, logger *zap.SugaredLogger, ctx context.Context, listener net.Listener, cfg *config.Config) (robot.LocalRobot, weboptions.Options) {
	options := weboptions.New()
	options.Network.BindAddress = ""
	options.Network.Listener = listener

	// start robot
	robot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, robot, test.ShouldNotBeNil)
	return robot, options
}

// NewRobotClient creates a new robot client with a certain address
func NewRobotClient(t *testing.T, logger *zap.SugaredLogger, addr string) *client.RobotClient {
	dur := 100 * time.Millisecond

	// start robot client
	robotClient, err := client.New(
		context.Background(),
		addr,
		logger,
		client.WithRefreshEvery(dur),
		client.WithCheckConnectedEvery(5*dur),
		client.WithReconnectEvery(dur),
	)
	test.That(t, err, test.ShouldBeNil)
	return robotClient
}
