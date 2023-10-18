package module_test

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/utils"
)

// TODO(pre-merge) refactor this somewhat-shared function into a test_helpers esque file.
func connect(port string, logger golog.Logger) (robot.Robot, error) {
	connectCtx, cancelConn := context.WithTimeout(context.Background(), time.Second*30)
	defer cancelConn()
	for {
		dialCtx, dialCancel := context.WithTimeout(context.Background(), time.Millisecond*500)
		rc, err := client.New(dialCtx, "localhost:"+port, logger,
			client.WithDialOptions(rpc.WithForceDirectGRPC()),
			client.WithDisableSessions(), // TODO(PRODUCT-343): add session support to modules
		)
		dialCancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			return rc, err
		}
		select {
		case <-connectCtx.Done():
			return nil, connectCtx.Err()
		default:
		}
	}
}

// TODO(pre-merge) refactor this somewhat-shared function into a test_helpers esque file.
func modifyCfg(t *testing.T, cfgIn string, logger golog.Logger) (string, string, error) {
	p, err := goutils.TryReserveRandomPort()
	if err != nil {
		return "", "", err
	}
	port := strconv.Itoa(p)

	cfg, err := config.Read(context.Background(), cfgIn, logger)
	if err != nil {
		return "", "", err
	}
	cfg.Network.BindAddress = "localhost:" + port
	output, err := json.Marshal(cfg)
	if err != nil {
		return "", "", err
	}
	file, err := os.CreateTemp(t.TempDir(), "viam-multiversionmodule-config-*")
	if err != nil {
		return "", "", err
	}
	cfgFilename := file.Name()
	_, err = file.Write(output)
	if err != nil {
		return "", "", err
	}
	return cfgFilename, port, file.Close()
}

func TestValidationFailureDuringReconfiguration(t *testing.T) {
	logger, logs := golog.NewObservedTestLogger(t)

	// Use a valid config
	cfgFilename, port, err := modifyCfg(t, utils.ResolveFile("module/multiversionmodule/module.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	serverPath, err := testutils.BuildTempModule(t, "web/cmd/server/")
	test.That(t, err, test.ShouldBeNil)

	server := pexec.NewManagedProcess(pexec.ProcessConfig{
		Name: serverPath,
		Args: []string{"-config", cfgFilename},
		CWD:  utils.ResolveFile("./"),
		Log:  true,
	}, logger)

	err = server.Start(context.Background())
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, server.Stop(), test.ShouldBeNil)
	}()

	rc, err := connect(port, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, rc.Close(context.Background()), test.ShouldBeNil)
	}()

	// Assert that motors and base were added.
	_, err = rc.ResourceByName(generic.Named("generic1"))
	test.That(t, err, test.ShouldBeNil)

	// Assert that there were no validation or component building errors
	test.That(t, logs.FilterMessageSnippet(
		"modular config validation error found in resource: generic1").Len(), test.ShouldEqual, 0)
	test.That(t, logs.FilterMessageSnippet("error building component").Len(), test.ShouldEqual, 0)

	// Read the config, swap to `run_version2.sh`, and overwrite the config, triggering a reconfigure where `generic1` will fail validation
	cfg, err := config.Read(context.Background(), cfgFilename, logger)
	test.That(t, err, test.ShouldBeNil)
	cfg.Modules[0].ExePath = utils.ResolveFile("module/multiversionmodule/run_version2.sh")
	newCfgBytes, err := json.Marshal(cfg)
	test.That(t, err, test.ShouldBeNil)
	os.WriteFile(cfgFilename, newCfgBytes, 0o644)

	// Wait for reconfiguration to finish with a 30 second timeout.
	reconfigureCheckTicker := time.NewTicker(time.Second)
	reconfigureFinished := false
	reconfigureCtx, reconfigureCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer reconfigureCancel()
	for !reconfigureFinished {
		select {
		case <-reconfigureCheckTicker.C:
			_, err = rc.ResourceByName(generic.Named("generic1"))
			if err != nil {
				reconfigureFinished = true
			}
		case <-reconfigureCtx.Done():
			test.That(t, reconfigureCtx.Err(), test.ShouldBeNil)
		}
	}

	// Check that the motors are still present but that "base1" is not started
	_, err = rc.ResourceByName(generic.Named("generic1"))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldResemble, `resource "rdk:component:generic/generic1" not found`)

	// Assert that Validation failure message is present
	//
	// Race condition safety: Resource removal should occur after modular resource validation (during completeConfig), so if
	// ResourceByName is failing, these errors should already be present
	test.That(t, logs.FilterMessageSnippet(
		"modular config validation error found in resource: generic1").Len(), test.ShouldEqual, 1)
	test.That(t, logs.FilterMessageSnippet("error building component").Len(), test.ShouldEqual, 0)
}

func TestVersionBumpWithNewImplicitDeps(t *testing.T) {
	logger, logs := golog.NewObservedTestLogger(t)

	// Use a valid config
	cfgFilename, port, err := modifyCfg(t, utils.ResolveFile("module/multiversionmodule/module.json"), logger)
	test.That(t, err, test.ShouldBeNil)

	serverPath, err := testutils.BuildTempModule(t, "web/cmd/server/")
	test.That(t, err, test.ShouldBeNil)

	server := pexec.NewManagedProcess(pexec.ProcessConfig{
		Name: serverPath,
		Args: []string{"-config", cfgFilename},
		CWD:  utils.ResolveFile("./"),
		Log:  true,
	}, logger)

	err = server.Start(context.Background())
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, server.Stop(), test.ShouldBeNil)
	}()

	rc, err := connect(port, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, rc.Close(context.Background()), test.ShouldBeNil)
	}()

	// Assert that motors and base were added.
	_, err = rc.ResourceByName(generic.Named("generic1"))
	test.That(t, err, test.ShouldBeNil)

	// Assert that there were no validation or component building errors
	test.That(t, logs.FilterMessageSnippet(
		"modular config validation error found in resource: generic1").Len(), test.ShouldEqual, 0)
	test.That(t, logs.FilterMessageSnippet("error building component").Len(), test.ShouldEqual, 0)

	// Read the config, swap to `run_version3.sh`, and overwrite the config. Version 3 requires
	// `generic1` to have a `motor` in its attributes. This config change should result in
	// `generic1` becoming unavailable.
	cfg, err := config.Read(context.Background(), cfgFilename, logger)
	test.That(t, err, test.ShouldBeNil)
	cfg.Modules[0].ExePath = utils.ResolveFile("module/multiversionmodule/run_version3.sh")
	newCfgBytes, err := json.Marshal(cfg)
	test.That(t, err, test.ShouldBeNil)
	os.WriteFile(cfgFilename, newCfgBytes, 0o644)

	// Wait for reconfiguration to finish with a 30 second timeout.
	reconfigureCheckTicker := time.NewTicker(time.Second)
	reconfigureFinished := false
	reconfigureCtx, reconfigureCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer reconfigureCancel()
	for !reconfigureFinished {
		select {
		case <-reconfigureCheckTicker.C:
			_, err = rc.ResourceByName(generic.Named("generic1"))
			if err != nil {
				reconfigureFinished = true
			}
		case <-reconfigureCtx.Done():
			test.That(t, reconfigureCtx.Err(), test.ShouldBeNil)
		}
	}
	reconfigureCancel()

	_, err = rc.ResourceByName(generic.Named("generic1"))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldResemble, `resource "rdk:component:generic/generic1" not found`)

	// Assert that Validation failure message is present
	//
	// Race condition safety: Resource removal should occur after modular resource validation (during completeConfig), so if
	// ResourceByName is failing, these errors should already be present
	test.That(t, logs.FilterMessageSnippet(
		"modular config validation error found in resource: generic1").Len(), test.ShouldEqual, 1)
	test.That(t, logs.FilterMessageSnippet("error building component").Len(), test.ShouldEqual, 0)

	// Update the generic1 configuration to have a `motor` attribute. The following reconfiguration
	// round should make the `generic1` component available again.
	for i, c := range cfg.Components {
		if c.Name == "generic1" {
			cfg.Components[i].Attributes = map[string]interface{}{"motor": "motor1"}
		}
	}

	newCfgBytes, err = json.Marshal(cfg)
	test.That(t, err, test.ShouldBeNil)
	os.WriteFile(cfgFilename, newCfgBytes, 0o644)

	// Wait for reconfiguration to finish with a 30 second timeout.
	reconfigureCheckTicker = time.NewTicker(time.Second)
	reconfigureFinished = false
	reconfigureCtx, reconfigureCancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer reconfigureCancel()
	for !reconfigureFinished {
		select {
		case <-reconfigureCheckTicker.C:
			_, err = rc.ResourceByName(generic.Named("generic1"))
			if err == nil {
				reconfigureFinished = true
			}
		case <-reconfigureCtx.Done():
			test.That(t, reconfigureCtx.Err(), test.ShouldBeNil)
		}
	}
}
