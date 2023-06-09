// Package robottestutils provides helper functions in testing
package robottestutils

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.uber.org/zap"
	genericpb "go.viam.com/api/component/generic/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.viam.com/rdk/config"
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
func NewRobotClient(tb testing.TB, logger *zap.SugaredLogger, addr string, dur time.Duration) *client.RobotClient {
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

func Connect(port string) (robotpb.RobotServiceClient, genericpb.GenericServiceClient, *grpc.ClientConn, error) {
	ctxTimeout, cancelFunc := context.WithTimeout(context.Background(), time.Minute)
	defer cancelFunc()

	var conn *grpc.ClientConn
	conn, err := grpc.DialContext(ctxTimeout,
		"dns:///localhost:"+port,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, nil, nil, err
	}

	rc := robotpb.NewRobotServiceClient(conn)
	gc := genericpb.NewGenericServiceClient(conn)

	return rc, gc, conn, nil
}

func MakeTempConfig(t *testing.T, cfg *config.Config, logger golog.Logger) (string, error) {
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
