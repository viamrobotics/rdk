// Package server implements the entry point for running a robot web server.
package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/invopop/jsonschema"
	"go.uber.org/zap/zapcore"
	pb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	gtestutils "go.viam.com/utils/testutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	configtestutils "go.viam.com/rdk/config/testutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
	"go.viam.com/rdk/testutils/robottestutils/serverutils"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/web/server"
)

func TestEntrypoint(t *testing.T) {
	ctx := context.Background()
	t.Run("number of resources", func(t *testing.T) {
		logger, logObserver := logging.NewObservedTestLogger(t)
		cfgFilename := utils.ResolveFile("/etc/configs/fake.json")
		cfg, err := config.Read(context.Background(), cfgFilename, logger, nil)
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

		rc, err := client.New(ctx, fmt.Sprintf("localhost:%d", port), logger)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, rc.Close(ctx), test.ShouldBeNil)
		}()

		resourceNames := rc.ResourceNames()

		// numResources is the # of resources in /etc/configs/fake.json + the 1
		// expected builtin resources.
		numResources := 21
		if runtime.GOOS == "windows" {
			// windows build excludes builtin models that use cgo,
			// including builtin motion, fake arm, and builtin navigation.
			numResources = 18
		}

		test.That(t, len(resourceNames), test.ShouldEqual, numResources)
	})
	t.Run("dump resource registrations", func(t *testing.T) {
		tempDir := t.TempDir()
		outputFile := filepath.Join(tempDir, "resources.json")
		serverPath := testutils.BuildViamServer(t)
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

		numReg := 53
		if runtime.GOOS == "windows" {
			// windows build excludes builtin models that use cgo
			numReg = 45
		}
		test.That(t, registrations, test.ShouldHaveLength, numReg)

		observedReg := make(map[string]bool)
		for _, reg := range registrations {
			test.That(t, reg.API, test.ShouldNotBeEmpty)
			test.That(t, reg.Model, test.ShouldNotBeEmpty)
			test.That(t, reg.Schema, test.ShouldNotBeNil)

			regStr := strings.Join([]string{reg.API, reg.Model}, "/")
			observedReg[regStr] = true
		}

		// Check specifically for registrations we care about
		expectedReg := []string{
			"rdk:component:arm/rdk:builtin:wrapper_arm",
			"rdk:service:data_manager/rdk:builtin:builtin",
			"rdk:service:shell/rdk:builtin:builtin",
			"rdk:service:vision/rdk:builtin:mlmodel",
		}

		// windows build excludes builtin models that use cgo, so add more if not
		// on windows
		if runtime.GOOS != "windows" {
			expectedReg = append(
				expectedReg,
				"rdk:component:camera/rdk:builtin:webcam",
				"rdk:service:motion/rdk:builtin:builtin",
			)
		}
		for _, reg := range expectedReg {
			test.That(t, observedReg[reg], test.ShouldBeTrue)
		}
	})
}

func TestShutdown(t *testing.T) {
	t.Run("shutdown functionality", func(t *testing.T) {
		testLogger := logging.NewTestLogger(t)
		// Pass in a separate logger to the managed server process that only outputs WARN+
		// logs. This avoids the test spamming stdout with stack traces from the shutdown command.
		serverLogger := logging.NewInMemoryLogger(t)
		serverLogObserver := serverLogger.Observer

		cfgFilename := utils.ResolveFile("/etc/configs/fake.json")
		cfg, err := config.Read(context.Background(), cfgFilename, testLogger, nil)
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

			// Start the server w/ ManagedProcess auto-restart disabled, otherwise
			// we'll be racing the process restart to check that the stop command
			// actually worked.
			server = robottestutils.ServerAsSeparateProcess(
				t,
				cfgFilename,
				serverLogger,
				robottestutils.WithoutRestart(),
			)
			err = server.Start(context.Background())
			test.That(t, err, test.ShouldBeNil)

			if success = robottestutils.WaitForServing(serverLogObserver, port); success {
				break
			}
			testLogger.Infow("Port in use. Restarting on new port.", "port", port, "err", err)
			server.Stop()
			continue
		}
		test.That(t, success, test.ShouldBeTrue)

		addr := "127.0.0.1:" + strconv.Itoa(port)
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
	// NOTE: not parallel — this test redirects the global utils.ViamDotDir, which would race
	// with other parallel tests that construct robots.
	logger := logging.NewTestLogger(t)
	ctx, cancel := context.WithCancel(context.Background())

	machineAddress := "127.0.0.1:23654"

	// Create a fake package directory and set it up to be identical to the expected file tree of
	// the local package manager: a single file `foo` in a `fake-module` directory. The local
	// package manager stores packages under <viam home>/packages-local, and the server (started
	// via RunServer below) uses the global utils.ViamDotDir as its home dir, so redirect that to
	// this temp dir to point it here.
	//
	// Use a short temp dir (not t.TempDir, whose Windows path is long) because redirecting
	// ViamDotDir also relocates the module socket dir on Windows, and the unix socket path has a
	// 103-char OS limit (see module.CreateSocketAddress).
	tempDir, err := os.MkdirTemp("", "vds")
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() { goutils.UncheckedError(os.RemoveAll(tempDir)) })
	origViamDotDir := utils.ViamDotDir
	utils.ViamDotDir = tempDir
	t.Cleanup(func() { utils.ViamDotDir = origViamDotDir })
	fakePackagePath := filepath.Join(tempDir, fmt.Sprint("packages", config.LocalPackagesSuffix))
	fakeModuleDataPath := filepath.Join(fakePackagePath, "data", "fake-module")
	err = os.MkdirAll(fakeModuleDataPath, 0o777) // should create all dirs along path
	test.That(t, err, test.ShouldBeNil)
	fakeModuleDataFile, err := os.Create(filepath.Join(fakeModuleDataPath, "foo"))
	test.That(t, err, test.ShouldBeNil)

	fakeDataFileName := fakeModuleDataFile.Name()
	test.That(t, fakeModuleDataFile.Close(), test.ShouldBeNil)

	// Register a slow-constructing generic resource and defer its deregistration.
	type slow struct {
		resource.Named
		resource.AlwaysRebuild
		resource.TriviallyCloseable
	}
	completeConstruction := make(chan struct{}, 1)
	slowModel := resource.NewModel("slow", "to", "build")
	resource.RegisterComponent(generic.API, slowModel, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (resource.Resource, error) {
			// Wait for `completeConstruction` to close before returning from constructor.
			<-completeConstruction

			return &slow{
				Named: conf.ResourceName().AsNamed(),
			}, nil
		},
	})
	defer func() {
		resource.Deregister(generic.API, slowModel)
	}()

	// Run entrypoint code (RunServer) in a goroutine, as it is blocking.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		cfg := &config.Config{
			Components: []resource.Config{
				{
					Name:  "slowpoke",
					API:   generic.API,
					Model: slowModel,
				},
			},
			Network: config.NetworkConfig{
				NetworkConfigData: config.NetworkConfigData{
					BindAddress: machineAddress,
				},
			},
		}
		tempConfigFileName, err := robottestutils.MakeTempConfig(t, cfg, logger)
		test.That(t, err, test.ShouldBeNil)

		args := []string{"viam-server", "-config", tempConfigFileName}
		test.That(t, server.RunServer(ctx, args, logger), test.ShouldBeNil)
	}()

	rc := robottestutils.NewRobotClient(t, logger, machineAddress, time.Second, client.WithDoNotWaitForRunning())

	// Assert that, from client's perspective, robot is in an initializing state until
	// `slowpoke` completes construction.
	machineStatus, err := rc.MachineStatus(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, machineStatus, test.ShouldNotBeNil)
	test.That(t, machineStatus.State, test.ShouldEqual, robot.StateInitializing)

	// Assert that the `foo` package file exists during initialization, machine assumes
	// package files may still be in use.)
	_, err = os.Stat(fakeDataFileName)
	test.That(t, err, test.ShouldBeNil)

	// Allow `slowpoke` to complete construction.
	close(completeConstruction)

	gtestutils.WaitForAssertion(t, func(tb testing.TB) {
		machineStatus, err := rc.MachineStatus(ctx)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, machineStatus, test.ShouldNotBeNil)
		test.That(tb, machineStatus.State, test.ShouldEqual, robot.StateRunning)
	})

	// Assert that the `foo` file was removed, as the non-initializing `Reconfigure`
	// determined it was unnecessary (no associated package/module.)
	_, err = os.Stat(fakeDataFileName)
	test.That(t, os.IsNotExist(err), test.ShouldBeTrue)

	// Cancel context and wait for server goroutine to stop running.
	cancel()
	wg.Wait()
}

func TestMachineStateNoResources(t *testing.T) {
	t.Parallel()
	// Regression test for RSDK-10166. Ensure that starting a robot with no resources will
	// still allow moving from initializing -> running state.

	logger := logging.NewTestLogger(t)
	ctx, cancel := context.WithCancel(context.Background())

	machineAddress := "127.0.0.1:23655"

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		cfg := &config.Config{
			Network: config.NetworkConfig{
				NetworkConfigData: config.NetworkConfigData{
					BindAddress: machineAddress,
				},
			},
		}
		tempConfigFileName, err := robottestutils.MakeTempConfig(t, cfg, logger)
		test.That(t, err, test.ShouldBeNil)

		args := []string{"viam-server", "-config", tempConfigFileName}
		test.That(t, server.RunServer(ctx, args, logger), test.ShouldBeNil)
	}()

	rc := robottestutils.NewRobotClient(t, logger, machineAddress, time.Second)

	// Assert that, from client's perspective, robot is in a running state since
	// `NewRobotClient` will only return at that point. We do not want to be stuck in
	// `robot.StateInitializing` forever despite having no resources in our config.
	machineStatus, err := rc.MachineStatus(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, machineStatus, test.ShouldNotBeNil)
	test.That(t, machineStatus.State, test.ShouldEqual, robot.StateRunning)

	// Cancel context and wait for server goroutine to stop running.
	cancel()
	wg.Wait()
}

func TestTunnelE2E(t *testing.T) {
	t.Parallel()
	// `TestTunnelE2E` attempts to send "Hello, World!" across a tunnel. The tunnel is:
	//
	// test-process <-> source-listener <-> machine <-> dest-listener
	//
	// Ports are reserved dynamically.

	tunnelMsg := "Hello, World!"

	destPort, destListener, err := goutils.ReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	destListenerAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(destPort))
	defer func() {
		test.That(t, destListener.Close(), test.ShouldBeNil)
	}()

	// Mock listener for the timeout endpoint — windows requires a real listener
	// even though we never accept on it.
	timeoutDestPort, timeoutDestListener, err := goutils.ReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, timeoutDestListener.Close(), test.ShouldBeNil)
	}()

	sourcePort, sourceListener, err := goutils.ReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)
	sourceListenerAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(sourcePort))
	defer func() {
		test.That(t, sourceListener.Close(), test.ShouldBeNil)
	}()

	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Infof("Listening on %s for tunnel message", destListenerAddr)
		conn, err := destListener.Accept()
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, conn.Close(), test.ShouldBeNil)
		}()

		bytes := make([]byte, 1024)
		n, err := conn.Read(bytes)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, n, test.ShouldEqual, len(tunnelMsg))
		test.That(t, string(bytes), test.ShouldContainSubstring, tunnelMsg)
		logger.Info("Received expected tunnel message at", destListenerAddr)

		// Write the same message back.
		n, err = conn.Write([]byte(tunnelMsg))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, n, test.ShouldEqual, len(tunnelMsg))
	}()

	// Use robottestutils helpers to avoid the TOCTOU port-bind race (RSDK-13776).
	cfg := &config.Config{
		Network: config.NetworkConfig{
			NetworkConfigData: config.NetworkConfigData{
				TrafficTunnelEndpoints: []config.TrafficTunnelEndpoint{
					{
						Port: destPort, // allow tunneling to destination port
					},
					{
						Port: timeoutDestPort, // allow tunneling to 65534
						// specify a negative timeout since somehow 1 ns succeeds on windows sometimes
						ConnectionTimeout: -time.Nanosecond,
					},
				},
			},
		},
	}
	rc, stopServer := serverutils.TryStartServerAndConnect(t, ctx, cfg, logger, nil)
	t.Cleanup(func() {
		test.That(t, rc.Close(ctx), test.ShouldBeNil)
		// stopServer will be called toward the end of the test so we can wait on the
		// background tunnel workers correctly.
	})

	// Test error paths for `Tunnel` with random `net.Conn`s.
	//
	// We will not be actually writing anything to/reading anything from the `net.Conn`, as
	// we only want to ensure that instantiation of the tunnel fails as expected.
	{
		googleConn, err := net.Dial("tcp", "google.com:443")
		test.That(t, err, test.ShouldBeNil)

		// Assert that opening a tunnel to a disallowed port errors.
		err = rc.Tunnel(ctx, googleConn /* will be eventually closed by `Tunnel` */, 404)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "tunnel not available at port")

		googleConn, err = net.Dial("tcp", "google.com:443")
		test.That(t, err, test.ShouldBeNil)

		// Assert that opening a tunnel to a port with a low `connection_timeout` results in a
		// timeout.
		err = rc.Tunnel(ctx, googleConn /* will be eventually closed by `Tunnel` */, timeoutDestPort)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "DeadlineExceeded")
	}

	// Start "source" listener (a `RobotClient` running `Tunnel`.)
	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Infof("Connections opened at %s will be tunneled", sourceListenerAddr)
		conn, err := sourceListener.Accept()
		test.That(t, err, test.ShouldBeNil)

		err = rc.Tunnel(ctx, conn /* will be eventually closed by `Tunnel` */, destPort)
		test.That(t, err, test.ShouldBeNil)
	}()

	// Write `tunnelMsg` to "source" listener over TCP from this test process.
	conn, err := net.Dial("tcp", sourceListenerAddr)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, conn.Close(), test.ShouldBeNil)
	}()
	n, err := conn.Write([]byte(tunnelMsg))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, n, test.ShouldEqual, len(tunnelMsg))

	// Expect `tunnelMsg` to be written back.
	bytes := make([]byte, 1024)
	n, err = conn.Read(bytes)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, n, test.ShouldEqual, len(tunnelMsg))
	test.That(t, string(bytes), test.ShouldContainSubstring, tunnelMsg)

	// Cancel the server context once the message has made it all the way across and
	// has been echoed back. This stops the RunServer goroutine.
	test.That(t, stopServer(), test.ShouldBeNil)

	wg.Wait()
}

// This is a regression test for RSDK-13456, which fixes a bug
// where viam-server would not enable debug logging on startup if
// debug: true was present in the config, but the config was file-based
func TestDebugLogAppliesAtStartup(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx, cancel := context.WithCancel(context.Background())

	// Start a machine with a testmodule and a 'helper' component that should start with
	// info-level logging.
	testModulePath := testutils.BuildTempModule(t, "module/testmodule")

	helperModel := resource.NewModel("rdk", "test", "helper")
	machineAddress := "127.0.0.1:23659"

	cfg := &config.Config{
		Modules: []config.Module{
			{
				Name:    "testModule",
				ExePath: testModulePath,
			},
		},
		Components: []resource.Config{
			{
				Name:  "helper",
				API:   generic.API,
				Model: helperModel,
			},
		},
		Network: config.NetworkConfig{
			NetworkConfigData: config.NetworkConfigData{
				BindAddress: machineAddress,
			},
		},
		Debug: true,
	}
	cfgFileName, err := robottestutils.MakeTempConfig(t, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	// Call `RunServer` in a goroutine as it is blocking. Point it to the temporary config
	// file created above.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		args := []string{"viam-server", "-config", cfgFileName}
		test.That(t, server.RunServer(ctx, args, logger), test.ShouldBeNil)
	}()

	// Create an SDK client to the server that was started on 127.0.0.1:23659.
	rc := robottestutils.NewRobotClient(t, logger, machineAddress, time.Second)
	t.Log(rc.ResourceNames())
	helper, err := rc.ResourceByName(generic.Named("helper"))
	test.That(t, err, test.ShouldBeNil)

	// Log a DEBUG line through helper. While we cannot actually examine the log output, we
	// can examine the response from the component to see its set log level. That level
	// should start as "Debug."
	resp, err := helper.DoCommand(ctx,
		map[string]any{"command": "log", "msg": "debug log line", "level": "DEBUG"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]any{"level": "Debug"})

	// Cancel context and wait for server goroutine to stop running.
	cancel()
	wg.Wait()
}

// TestCloudModulesRespondToDebugAndLogChanges is the cloud-config variant of
// TestModulesRespondToDebugAndLogChanges. It verifies that toggling the top-level
// "debug" flag through a cloud config correctly propagates to modules AND that
// modules are not spuriously restarted a second time when the (unchanged) cloud
// config is re-fetched on the next poll cycle.
func TestCloudModulesRespondToDebugAndLogChanges(t *testing.T) {
	t.Parallel()

	logger := logging.NewTestLogger(t)
	ctx, cancel := context.WithCancel(context.Background())

	testModulePath := testutils.BuildTempModule(t, "module/testmodule")
	helperModel := resource.NewModel("rdk", "test", "helper")

	// Set up fake cloud server.
	fakeServer, cleanup := configtestutils.NewFakeCloudServer(t, ctx, logger)
	defer cleanup()

	deviceID := "test-device-id"
	appAddress := fmt.Sprintf("http://%s", fakeServer.Addr().String())

	// Build the base proto config pieces that remain constant across updates.
	// Note: AppAddress and RefreshInterval are deliberately NOT set here.
	// pb.CloudConfig (and thus CloudConfigToProto) does not carry them — they are
	// local bootstrap-only fields. They are set directly on baseConfigNonProto
	// below so the in-process server can reach the fake cloud and poll at the
	// expected cadence.
	cloudConfProto, err := config.CloudConfigToProto(&config.Cloud{
		ID:                deviceID,
		Secret:            configtestutils.FakeCredentialPayLoad,
		FQDN:              "woo",
		LocalFQDN:         "yee",
		SignalingInsecure: true,
		PrimaryOrgID:      "the-primary-org",
		LocationID:        "the-location",
		MachineID:         "the-machine",
		LocationSecrets:   []config.LocationSecret{{ID: "1", Secret: "secret"}},
	})
	test.That(t, err, test.ShouldBeNil)

	moduleProto, err := config.ModuleConfigToProto(&config.Module{
		Name:    "testModule",
		ExePath: testModulePath,
	})
	test.That(t, err, test.ShouldBeNil)

	componentProto, err := config.ComponentConfigToProto(&resource.Config{
		Name:  "helper",
		API:   generic.API,
		Model: helperModel,
	})
	test.That(t, err, test.ShouldBeNil)

	baseConfig := &pb.RobotConfig{
		Cloud:      cloudConfProto,
		Modules:    []*pb.ModuleConfig{moduleProto},
		Components: []*pb.ComponentConfig{componentProto},
	}

	// storeCloudConfig clones cfg and stores the clone in the fake cloud server.
	// This avoids a data race between the test goroutine mutating baseConfig
	// and the gRPC handler goroutine marshaling the stored proto.
	storeCloudConfig := func(cfg *pb.RobotConfig) {
		t.Helper()
		fakeServer.StoreDeviceConfig(deviceID, proto.Clone(cfg).(*pb.RobotConfig), &pb.CertificateResponse{})
	}

	// The initial config uses debug=false. The bind address (Network) is set on
	// baseConfig and stored in the server-startup retry loop below.
	debugFalse := false
	baseConfig.Debug = &debugFalse

	// Use robottestutils helpers to avoid the TOCTOU port-bind race (RSDK-13776).
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	test.That(t, err, test.ShouldBeNil)
	machineAddress := listener.Addr().String()
	test.That(t, listener.Close(), test.ShouldBeNil)

	networkProto, err := config.NetworkConfigToProto(&config.NetworkConfig{
		NetworkConfigData: config.NetworkConfigData{BindAddress: machineAddress},
	})
	test.That(t, err, test.ShouldBeNil)
	baseConfig.Network = networkProto
	storeCloudConfig(baseConfig)

	baseConfigNonProto, err := config.FromProto(baseConfig, logger)
	test.That(t, err, test.ShouldBeNil)

	// AppAddress and RefreshInterval are local bootstrap-only cloud fields that do
	// not round-trip through pb.CloudConfig, so set them on the config the server
	// actually boots from. Without AppAddress the server cannot reach the fake
	// cloud to fetch its config.
	baseConfigNonProto.Cloud.AppAddress = appAddress
	// shorten the refresh interval to make the test run faster
	refreshInterval := 1 * time.Second
	baseConfigNonProto.Cloud.RefreshInterval = refreshInterval

	rc, stopServer := serverutils.TryStartServerAndConnect(
		t,
		ctx,
		baseConfigNonProto,
		logger,
		// Cloud-configured servers require location-secret auth. Use -no-tls as an extra
		// viam-server flag since the fake cloud returns empty TLS certs.
		[]string{"-no-tls"},
		client.WithDialOptions(
			rpc.WithEntityCredentials("woo", rpc.Credentials{
				Type:    utils.CredentialsTypeRobotLocationSecret,
				Payload: "secret",
			}),
			rpc.WithAllowInsecureWithCredentialsDowngrade(),
		),
	)
	t.Cleanup(func() {
		test.That(t, rc.Close(ctx), test.ShouldBeNil)
		test.That(t, stopServer(), test.ShouldBeNil)
	})

	// helper function that waits longer than the specified refreshInterval
	// to make sure we always wait long enough for a reconfigure to happen
	waitForAssertionLongerThanRefreshInterval := func(t *testing.T, assertion func(tb testing.TB)) {
		t.Helper()
		retryInterval := 50 * time.Millisecond
		nRuns := int(refreshInterval * 3 / retryInterval)
		gtestutils.WaitForAssertionWithSleep(t, retryInterval, nRuns, assertion)
	}

	var helper resource.Resource
	waitForAssertionLongerThanRefreshInterval(t, func(tb testing.TB) {
		helper, err = rc.ResourceByName(generic.Named("helper"))
		test.That(tb, err, test.ShouldBeNil)
	})

	// Verify initial state: helper should report Info-level logging.
	waitForAssertionLongerThanRefreshInterval(t, func(tb testing.TB) {
		resp, err := helper.DoCommand(ctx,
			map[string]any{"command": "log", "msg": "debug log line", "level": "DEBUG"})
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, resp, test.ShouldResemble, map[string]any{"level": "Info"})
	})

	// --- Transition: debug false → true ---
	debugTrue := true
	baseConfig.Debug = &debugTrue
	storeCloudConfig(baseConfig)

	// Wait for the helper to report Debug-level logging.
	waitForAssertionLongerThanRefreshInterval(t, func(tb testing.TB) {
		resp, err := helper.DoCommand(ctx,
			map[string]any{"command": "log", "msg": "debug log line", "level": "DEBUG"})
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, resp, test.ShouldResemble, map[string]any{"level": "Debug"})
	})

	// --- Transition: debug true → false ---
	baseConfig.Debug = &debugFalse
	storeCloudConfig(baseConfig)

	// Wait for the helper to report Info-level logging again.
	waitForAssertionLongerThanRefreshInterval(t, func(tb testing.TB) {
		resp, err := helper.DoCommand(ctx,
			map[string]any{"command": "log", "msg": "debug log line", "level": "DEBUG"})
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, resp, test.ShouldResemble, map[string]any{"level": "Info"})
	})

	// --- Transition: add log pattern "testModule" at debug level ---
	baseConfig.Log = []*pb.LogPatternConfig{{Pattern: "testModule", Level: "debug"}}
	storeCloudConfig(baseConfig)

	// Wait for the helper to report Debug-level logging.
	waitForAssertionLongerThanRefreshInterval(t, func(tb testing.TB) {
		resp, err := helper.DoCommand(ctx,
			map[string]any{"command": "log", "msg": "debug log line", "level": "DEBUG"})
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, resp, test.ShouldResemble, map[string]any{"level": "Debug"})
	})

	// --- Transition: remove log pattern ---
	baseConfig.Log = nil
	storeCloudConfig(baseConfig)

	// Wait for the helper to report Info-level logging again.
	waitForAssertionLongerThanRefreshInterval(t, func(tb testing.TB) {
		resp, err := helper.DoCommand(ctx,
			map[string]any{"command": "log", "msg": "debug log line", "level": "DEBUG"})
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, resp, test.ShouldResemble, map[string]any{"level": "Info"})
	})

	cancel()
}
