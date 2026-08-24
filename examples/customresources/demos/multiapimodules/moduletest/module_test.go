// Package main tests the multiapimodules demo: a composite (multi-API) model served by one module
// and a consumer served by a second module that depends on every one of the composite's APIs.
package main_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
	"go.viam.com/rdk/utils"
)

func TestMultiAPIModules(t *testing.T) {
	logger, observer := logging.NewObservedTestLogger(t)
	var port int
	success := false
	for portTryNum := 0; portTryNum < 10; portTryNum++ {
		cfgFilename, portLocal, err := modifyCfg(t,
			utils.ResolveFile("examples/customresources/demos/multiapimodules/module.json"), logger)
		test.That(t, err, test.ShouldBeNil)
		port = portLocal

		server := robottestutils.ServerAsSeparateProcess(t, cfgFilename, logger)
		err = server.Start(context.Background())
		test.That(t, err, test.ShouldBeNil)

		if robottestutils.WaitForServing(observer, port) {
			success = true
			defer func() { test.That(t, server.Stop(), test.ShouldBeNil) }()
			break
		}
		server.Stop()
	}
	test.That(t, success, test.ShouldBeTrue)

	rc, err := connect(port, logger, rpc.WithForceDirectGRPC())
	test.That(t, err, test.ShouldBeNil)
	defer func() { test.That(t, rc.Close(context.Background()), test.ShouldBeNil) }()

	const comboName = "combodev1"

	// The composite is reachable under each of its three APIs, and a single handle exposes all three.
	t.Run("composite reachable under every API", func(t *testing.T) {
		gizRes, err := rc.ResourceByName(gizmoapi.Named(comboName))
		test.That(t, err, test.ShouldBeNil)

		giz, err := resource.AsType[gizmoapi.Gizmo](gizRes)
		test.That(t, err, test.ShouldBeNil)
		two, err := giz.DoTwo(context.Background(), true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, two, test.ShouldEqual, "combo saw arg1=true")

		sum, err := resource.AsType[summationapi.Summation](gizRes)
		test.That(t, err, test.ShouldBeNil)
		total, err := sum.Sum(context.Background(), []float64{1, 2, 3, 4})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, total, test.ShouldEqual, 10)

		sen, err := resource.AsType[sensor.Sensor](gizRes)
		test.That(t, err, test.ShouldBeNil)
		readings, err := sen.Readings(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readings["reading"], test.ShouldEqual, 42)
	})

	// The consumer, in a separate module, depends on the composite and drives all three of its APIs.
	t.Run("consumer drives all three APIs of the dependency", func(t *testing.T) {
		userRes, err := rc.ResourceByName(generic.Named("user1"))
		test.That(t, err, test.ShouldBeNil)
		out, err := userRes.DoCommand(context.Background(), map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, out["gizmo_dotwo"], test.ShouldEqual, "combo saw arg1=true")
		test.That(t, out["sum"], test.ShouldEqual, 10)
		readings, ok := out["readings"].(map[string]interface{})
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, readings["reading"], test.ShouldEqual, 42)
	})
}

func connect(port int, logger logging.Logger, dialOpts ...rpc.DialOption) (robot.Robot, error) {
	connectCtx, cancelConn := context.WithTimeout(context.Background(), time.Second*30)
	defer cancelConn()
	for {
		dialCtx, dialCancel := context.WithTimeout(context.Background(), time.Second*2)
		rc, err := client.New(dialCtx, fmt.Sprintf("localhost:%d", port), logger,
			client.WithDialOptions(dialOpts...),
			client.WithDisableSessions(),
		)
		dialCancel()
		if !isRetryableConnErr(err) {
			return rc, err
		}
		select {
		case <-connectCtx.Done():
			return nil, connectCtx.Err()
		default:
		}
	}
}

func isRetryableConnErr(err error) bool {
	return err != nil && errors.Is(err, context.DeadlineExceeded)
}

func modifyCfg(t *testing.T, cfgIn string, logger logging.Logger) (string, int, error) {
	comboModPath := testutils.BuildTempModule(t, "examples/customresources/demos/multiapimodules/combomodule")
	userModPath := testutils.BuildTempModule(t, "examples/customresources/demos/multiapimodules/combousermodule")

	port, err := goutils.TryReserveRandomPort()
	if err != nil {
		return "", 0, err
	}

	cfg, err := config.Read(context.Background(), cfgIn, logger, nil)
	if err != nil {
		return "", 0, err
	}
	cfg.Network.BindAddress = fmt.Sprintf("localhost:%d", port)
	cfg.Modules[0].ExePath = comboModPath
	cfg.Modules[1].ExePath = userModPath
	output, err := json.Marshal(cfg)
	if err != nil {
		return "", 0, err
	}
	file, err := os.CreateTemp(t.TempDir(), "viam-test-config-*")
	if err != nil {
		return "", 0, err
	}
	cfgFilename := file.Name()
	if _, err = file.Write(output); err != nil {
		return "", 0, err
	}
	return cfgFilename, port, file.Close()
}
