// Package robottestutils provides helper functions in testing
package robottestutils

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"regexp"
	"testing"
	"time"

	"go.uber.org/zap/zaptest/observer"
	"go.viam.com/test"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/testutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot/client"
	weboptions "go.viam.com/rdk/robot/web/options"
	rtestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/utils"
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
	ctx := context.Background()
	robotClient, err := client.New(
		ctx,
		addr,
		logger,
		client.WithRefreshEvery(dur),
		client.WithCheckConnectedEvery(5*dur),
		client.WithReconnectEvery(dur),
	)
	test.That(tb, err, test.ShouldBeNil)
	tb.Cleanup(func() {
		test.That(tb, robotClient.Close(ctx), test.ShouldBeNil)
	})
	return robotClient
}

// Connect creates a new grpc.ClientConn server running on localhost:port.
func Connect(port int) (*grpc.ClientConn, error) {
	ctxTimeout, cancelFunc := context.WithTimeout(context.Background(), time.Minute)
	defer cancelFunc()

	var conn *grpc.ClientConn
	conn, err := grpc.DialContext(ctxTimeout,
		fmt.Sprintf("dns:///localhost:%d", port),
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

// ServerAsSeparateProcess builds the viam server and returns an unstarted ManagedProcess for
// the built binary.
func ServerAsSeparateProcess(t *testing.T, cfgFileName string, logger logging.Logger) pexec.ManagedProcess {
	serverPath := rtestutils.BuildTempModule(t, "web/cmd/server/")

	// use a temporary home directory so that it doesn't collide with
	// the user's/other tests' viam home directory
	testTempHome := t.TempDir()
	server := pexec.NewManagedProcess(pexec.ProcessConfig{
		Name:        serverPath,
		Args:        []string{"-config", cfgFileName},
		CWD:         utils.ResolveFile("./"),
		Environment: map[string]string{"HOME": testTempHome},
		Log:         true,
	}, logger.AsZap())
	return server
}

// WaitForServing will scan the logs in the `observer` input until seeing a "serving" or "error
// serving web" message. For added accuracy, it also checks that the port a test is expecting to
// start a server on matches the one in the log message.
//
// WaitForServing will return true if the server has started successfully in the allotted time, and
// false otherwise.
// nolint
func WaitForServing(observer *observer.ObservedLogs, port int) bool {
	// Message:"\n\\_ 2024-02-07T20:47:03.576Z\tINFO\trobot_server\tweb/web.go:598\tserving\t{\"url\":\"http://127.0.0.1:20000\"}"
	successRegex := regexp.MustCompile(fmt.Sprintf("\tserving\t.*:%d\"", port))
	// Message:"\n\\_ 2024-02-02T14:43:02.862Z\tERROR\trobot_server\tserver/entrypoint.go:177\terror serving web\t{\"error\":\"listen tcp 127.0.0.1:8090: bind: address already in use\"}"
	failRegex := regexp.MustCompile(fmt.Sprintf("\terror serving web\t.*:%d:", port))
	lastSeenLogIdx := 0
	for tryNum := 0; tryNum < 60; tryNum++ {
		// `ObservedLogs.All` does not "consume" the logs it is holding internally (whereas
		// `ObservedLogs.TakeAll` does). Some tests assert on logs that happen prior to serving on
		// an address. We could scan all the logs on each pass. But instead we introduce the
		// optimization of only scanning logs that were new since the last scan with
		// `lastSeenLogIdx`.
		newLogs := observer.All()[lastSeenLogIdx:]
		lastSeenLogIdx += len(newLogs)
		for _, log := range newLogs {
			switch {
			case successRegex.MatchString(log.Message):
				return true
			case failRegex.MatchString(log.Message):
				return false
			default:
			}
		}
		time.Sleep(time.Second)
	}

	return false
}
