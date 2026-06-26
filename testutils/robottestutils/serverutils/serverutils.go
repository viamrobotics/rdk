// Package serverutils provides test helpers that start an in-process
// viam-server. It is split out from the parent robottestutils package because
// it imports go.viam.com/rdk/web/server, which itself depends on robot/impl and
// robot/web; importing it from robottestutils directly would create an import
// cycle for those packages' tests.
package serverutils

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"go.viam.com/test"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/web/server"
)

// TryStartServerAndConnect attempts to start an in-process viam-server and
// connect a robot client to it, racing the connection against the server
// exiting early (e.g. from a bind-address conflict) so failures are detected
// promptly rather than hanging until a package-wide test timeout fires.
//
// The server is run via server.RunServer in a background goroutine using a
// temporary config file derived from cfg and any extraViamServerFlags. If
// cfg.Network.BindAddress is set, it is rewritten to a freshly reserved random
// port on 127.0.0.1 before each attempt.
//
// On success TryStartServerAndConnect returns (rc, stop) where stop() cancels
// the server context, waits for the server goroutine to exit, and returns the
// RunServer error. On failure it returns (nil, nil) and marks the test object
// as failed.
//
// connectOpts are appended to the default refresh/reconnect options passed to
// client.New. The connection attempt is bounded by a 30-second timeout.
//
// The entire sequence is attempted up to 3 times before declaring failure.
func TryStartServerAndConnect(
	t *testing.T,
	ctx context.Context,
	cfg *config.Config,
	logger logging.Logger,
	extraViamServerFlags []string,
	connectOpts ...client.RobotClientOption,
) (*client.RobotClient, func() error) {
	t.Helper()
	for attempt := 1; attempt <= 3; attempt++ {
		rc, stop := tryStartServerAndConnectInner(t, ctx, cfg, logger, extraViamServerFlags, connectOpts...)
		if rc != nil {
			return rc, stop
		}
	}
	t.Fatal("failed to start and connect to server")
	return nil, nil
}

func tryStartServerAndConnectInner(
	t *testing.T,
	ctx context.Context,
	cfg *config.Config,
	logger logging.Logger,
	extraViamServerFlags []string,
	connectOpts ...client.RobotClientOption,
) (*client.RobotClient, func() error) {
	t.Helper()

	machineAddr := cfg.Network.BindAddress
	if machineAddr == "" {
		machinePort, err := goutils.TryReserveRandomPort()
		test.That(t, err, test.ShouldBeNil)
		machineAddr = net.JoinHostPort("127.0.0.1", strconv.Itoa(machinePort))
		cfg.Network.BindAddress = machineAddr
	}

	tempConfigFile, err := os.CreateTemp(t.TempDir(), "temp_config.json")
	test.That(t, err, test.ShouldBeNil)
	tempConfigFileName := tempConfigFile.Name()
	test.That(t, tempConfigFile.Close(), test.ShouldBeNil)

	cfgBytes, err := json.Marshal(&cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, os.WriteFile(tempConfigFileName, cfgBytes, 0o755), test.ShouldBeNil)

	serverCtx, serverCancel := context.WithCancel(ctx)
	serverErrC := make(chan error, 1)
	var serverWg sync.WaitGroup
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		args := append([]string{"viam-server", "-config", tempConfigFileName}, extraViamServerFlags...)
		serverErrC <- server.RunServer(serverCtx, args, logger)
	}()

	type connectOutcome struct {
		rc  *client.RobotClient
		err error
	}

	connectCtx, connectCancel := context.WithTimeout(ctx, 30*time.Second)
	defer connectCancel()
	connectC := make(chan connectOutcome, 1)
	go func() {
		defaultOpts := []client.RobotClientOption{
			client.WithRefreshEvery(time.Second),
			client.WithCheckConnectedEvery(5 * time.Second),
			client.WithReconnectEvery(time.Second),
		}
		candidate, connErr := client.New(
			connectCtx, machineAddr, logger,
			append(defaultOpts, connectOpts...)...,
		)
		connectC <- connectOutcome{candidate, connErr}
	}()

	select {
	case outcome := <-connectC:
		if outcome.err == nil {
			stop := func() error {
				serverCancel()
				serverWg.Wait()
				return <-serverErrC
			}
			return outcome.rc, stop
		}
		logger.Infow("failed to connect to in-process viam-server; retrying on a new port", "err", outcome.err)
	case rsErr := <-serverErrC:
		logger.Infow("in-process viam-server exited before becoming reachable; retrying on a new port", "err", rsErr)
		// Unblock and wait for the in-flight connection attempt so it doesn't linger.
		connectCancel()
		<-connectC
	}

	serverCancel()
	serverWg.Wait()
	return nil, nil
}
