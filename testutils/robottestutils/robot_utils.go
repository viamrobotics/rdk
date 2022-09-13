// Package robottestutils provides helper functions in testing
package robottestutils

import (
	"context"
	"net"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/robot/client"
	weboptions "go.viam.com/rdk/robot/web/options"
)

// CreateBaseOptionsAndListener creates a new web options with random port as listener.
func CreateBaseOptionsAndListener(t *testing.T) (weboptions.Options, net.Listener, string) {
	t.Helper()
	var listener net.Listener = testutils.ReserveRandomListener(t)
	options := weboptions.New()
	options.Network.BindAddress = ""
	options.Network.Listener = listener
	addr := listener.Addr().String()
	return options, listener, addr
}

// NewRobotClient creates a new robot client with a certain address.
func NewRobotClient(t *testing.T, logger *zap.SugaredLogger, addr string, dur time.Duration) *client.RobotClient {
	t.Helper()
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
