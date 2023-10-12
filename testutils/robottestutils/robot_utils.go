// Package robottestutils provides helper functions in testing
package robottestutils

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot/client"
	weboptions "go.viam.com/rdk/robot/web/options"
)

// CreateBaseOptionsAndListener creates a new web options with random port as listener.
func CreateBaseOptionsAndListener(tb testing.TB) (weboptions.Options, net.Listener, string) {
	tb.Helper()
	var listener net.Listener = testutils.ReserveRandomListener(tb)
	options := weboptions.New()
	options.Network.BindAddress = ""
	options.Network.Listener = listener
	addr := listener.Addr().String()
	return options, listener, addr
}

// NewRobotClient creates a new robot client with a certain address.
func NewRobotClient(tb testing.TB, logger logging.Logger, addr string, dur time.Duration) *client.RobotClient {
	tb.Helper()
	// start robot client
	robotClient, err := client.New(
		context.Background(),
		addr,
		logger,
		client.WithRefreshEvery(dur),
		client.WithCheckConnectedEvery(5*dur),
		client.WithReconnectEvery(dur),
	)
	test.That(tb, err, test.ShouldBeNil)
	return robotClient
}

// Connect creates a new grpc.ClientConn server running on localhost:port.
func Connect(port string) (*grpc.ClientConn, error) {
	ctxTimeout, cancelFunc := context.WithTimeout(context.Background(), time.Minute)
	defer cancelFunc()

	var conn *grpc.ClientConn
	conn, err := grpc.DialContext(ctxTimeout,
		"dns:///localhost:"+port,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// MakeTempConfig writes a config.Config object to a temporary file for testing.
func MakeTempConfig(t *testing.T, cfg *config.Config, logger logging.Logger) (string, error) {
	if err := cfg.Ensure(false, logger); err != nil {
		return "", err
	}
	output, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	file, err := os.CreateTemp(t.TempDir(), "fake-*")
	if err != nil {
		return "", err
	}
	_, err = file.Write(output)
	if err != nil {
		return "", err
	}
	return file.Name(), file.Close()
}
