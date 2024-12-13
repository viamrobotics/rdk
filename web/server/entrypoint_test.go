// Package server implements the entry point for running a robot web server.
package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/invopop/jsonschema"
	"go.uber.org/zap/zapcore"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"
	gtestutils "go.viam.com/utils/testutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
	"go.viam.com/rdk/utils"
)

// numResources is the # of resources in /etc/configs/fake.json + the 2
// expected builtin resources.
const numResources = 20

func TestEntrypoint(t *testing.T) {
	if runtime.GOARCH == "arm" {
		t.Skip("skipping on 32-bit ARM, subprocess build warnings cause failure")
	}

	t.Run("number of resources", func(t *testing.T) {
		logger, logObserver := logging.NewObservedTestLogger(t)
		cfgFilename := utils.ResolveFile("/etc/configs/fake.json")
		cfg, err := config.Read(context.Background(), cfgFilename, logger)
		test.That(t, err, test.ShouldBeNil)

		var port int
		var success bool
		for portTryNum := 0; portTryNum < 10; portTryNum++ {
			p, err := goutils.TryReserveRandomPort()
			port = p
			test.That(t, err, test.ShouldBeNil)

			cfg.Network.BindAddress = fmt.Sprintf(":%d", port)
			cfgFilename, err = robottestutils.MakeTempConfig(t, cfg, logger)
			test.That(t, err, test.ShouldBeNil)

			server := robottestutils.ServerAsSeparateProcess(t, cfgFilename, logger)

			err = server.Start(context.Background())
			test.That(t, err, test.ShouldBeNil)

			if success = robottestutils.WaitForServing(logObserver, port); success {
				defer func() {
					test.That(t, server.Stop(), test.ShouldBeNil)
				}()
				break
			}
			logger.Infow("Port in use. Restarting on new port.", "port", port, "err", err)
			server.Stop()
			continue
		}
		test.That(t, success, test.ShouldBeTrue)

		conn, err := robottestutils.Connect(port)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, conn.Close(), test.ShouldBeNil)
		}()
		rc := robotpb.NewRobotServiceClient(conn)

		resourceNames, err := rc.ResourceNames(context.Background(), &robotpb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)

		test.That(t, len(resourceNames.Resources), test.ShouldEqual, numResources)
	})
	t.Run("dump resource registrations", func(t *testing.T) {
		tempDir := t.TempDir()
		outputFile := filepath.Join(tempDir, "resources.json")
		serverPath := testutils.BuildTempModule(t, "web/cmd/server/")
		command := exec.Command(serverPath, "--dump-resources", outputFile)
		err := command.Run()
		test.That(t, err, test.ShouldBeNil)
		type registration struct {
			Model  string             `json:"model"`
			API    string             `json:"API"`
			Schema *jsonschema.Schema `json:"attribute_schema"`
		}
		outputBytes, err := os.ReadFile(outputFile)
		test.That(t, err, test.ShouldBeNil)
		registrations := []registration{}
		err = json.Unmarshal(outputBytes, &registrations)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(registrations), test.ShouldBeGreaterThan, 0) // to protect against misreading resource registrations
		test.That(t, registrations, test.ShouldHaveLength, len(resource.RegisteredResources()))
		for _, reg := range registrations {
			test.That(t, reg.API, test.ShouldNotBeEmpty)
			test.That(t, reg.Model, test.ShouldNotBeEmpty)
			test.That(t, reg.Schema, test.ShouldNotBeNil)
		}
	})
}

func TestShutdown(t *testing.T) {
	if runtime.GOARCH == "arm" {
		t.Skip("skipping on 32-bit ARM, subprocess build warnings cause failure")
	}

	t.Run("shutdown functionality", func(t *testing.T) {
		testLogger := logging.NewTestLogger(t)
		// Pass in a separate logger to the managed server process that only outputs WARN+
		// logs. This avoids the test spamming stdout with stack traces from the shutdown command.
		serverLogger := logging.NewInMemoryLogger(t)
		serverLogObserver := serverLogger.Observer

		cfgFilename := utils.ResolveFile("/etc/configs/fake.json")
		cfg, err := config.Read(context.Background(), cfgFilename, testLogger)
		test.That(t, err, test.ShouldBeNil)

		var port int
		var success bool
		var server pexec.ManagedProcess
		for portTryNum := 0; portTryNum < 10; portTryNum++ {
			p, err := goutils.TryReserveRandomPort()
			port = p
			test.That(t, err, test.ShouldBeNil)

			cfg.Network.BindAddress = fmt.Sprintf(":%d", port)
			cfgFilename, err = robottestutils.MakeTempConfig(t, cfg, testLogger)
			test.That(t, err, test.ShouldBeNil)

			server = robottestutils.ServerAsSeparateProcess(t, cfgFilename, serverLogger)
			err = server.Start(context.Background())
			test.That(t, err, test.ShouldBeNil)

			if success = robottestutils.WaitForServing(serverLogObserver, port); success {
				defer func() {
					test.That(t, server.Stop(), test.ShouldBeNil)
				}()
				break
			}
			testLogger.Infow("Port in use. Restarting on new port.", "port", port, "err", err)
			server.Stop()
			continue
		}
		test.That(t, success, test.ShouldBeTrue)

		addr := "localhost:" + strconv.Itoa(port)
		rc := robottestutils.NewRobotClient(t, testLogger, addr, time.Second)

		testLogger.Info("Issuing shutdown.")
		err = rc.Shutdown(context.Background())

		gtestutils.WaitForAssertionWithSleep(t, 50*time.Millisecond, 50, func(tb testing.TB) {
			tb.Helper()
			rdkStatus := server.Status()
			// Asserting not nil here to ensure process is dead
			test.That(tb, rdkStatus, test.ShouldNotBeNil)
		})
		test.That(t, isExpectedShutdownError(err, testLogger), test.ShouldBeTrue)
		test.That(t, serverLogObserver.FilterLevelExact(zapcore.ErrorLevel).Len(), test.ShouldEqual, 0)
	})
}

func isExpectedShutdownError(err error, testLogger logging.Logger) bool {
	if err == nil {
		return true
	}

	expectedErrorCode := map[codes.Code]bool{
		codes.Unavailable:      true,
		codes.DeadlineExceeded: true,
	}
	if status, ok := status.FromError(err); ok && expectedErrorCode[status.Code()] {
		return true
	}

	testLogger.Errorw("Unexpected shutdown error", "err", err)
	return false
}

// Tests that machine state properly reports initializing or running.
func TestMachineState(t *testing.T) {
	if runtime.GOARCH == "arm" {
		t.Skip("skipping on 32-bit ARM, subprocess build warnings cause failure")
	}

	logger, logObserver := logging.NewObservedTestLogger(t)

	cfgFilename := utils.ResolveFile("/etc/configs/fake.json")
	cfg, err := config.Read(context.Background(), cfgFilename, logger)
	test.That(t, err, test.ShouldBeNil)

	var port int
	var success bool
	var server pexec.ManagedProcess
	for portTryNum := 0; portTryNum < 10; portTryNum++ {
		p, err := goutils.TryReserveRandomPort()
		port = p
		test.That(t, err, test.ShouldBeNil)

		cfg.Network.BindAddress = fmt.Sprintf(":%d", port)
		cfgFilename, err = robottestutils.MakeTempConfig(t, cfg, logger)
		test.That(t, err, test.ShouldBeNil)

		server = robottestutils.ServerAsSeparateProcess(t, cfgFilename, logger)
		err = server.Start(context.Background())
		test.That(t, err, test.ShouldBeNil)

		if success = robottestutils.WaitForServing(logObserver, port); success {
			defer func() {
				test.That(t, server.Stop(), test.ShouldBeNil)
			}()
			break
		}
		logger.Infow("Port in use. Restarting on new port.", "port", port, "err", err)
		server.Stop()
		continue
	}
	test.That(t, success, test.ShouldBeTrue)

	addr := "localhost:" + strconv.Itoa(port)
	rc := robottestutils.NewRobotClient(t, logger, addr, time.Second)

	// NewRobotClient will wait for machine state to be running. Assert that this
	// is still the case.
	machineStatus, err := rc.MachineStatus(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, machineStatus, test.ShouldNotBeNil)
	test.That(t, machineStatus.State, test.ShouldEqual, robot.StateRunning)
}
