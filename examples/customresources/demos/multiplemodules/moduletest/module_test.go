// Package main tests out all the custom models in the multiplemodules.
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

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
	"go.viam.com/rdk/utils"
)

func TestMultipleModules(t *testing.T) {
	logger, observer := logging.NewObservedTestLogger(t)

	var port int
	success := false
	for portTryNum := 0; portTryNum < 10; portTryNum++ {
		// Modify the example config to run directly, without compiling the module first.
		cfgFilename, portLocal, err := modifyCfg(t, utils.ResolveFile("examples/customresources/demos/multiplemodules/module.json"), logger)
		port = portLocal
		test.That(t, err, test.ShouldBeNil)

		server := robottestutils.ServerAsSeparateProcess(t, cfgFilename, logger)

		err = server.Start(context.Background())
		test.That(t, err, test.ShouldBeNil)

		if robottestutils.WaitForServing(observer, port) {
			success = true
			defer func() {
				test.That(t, server.Stop(), test.ShouldBeNil)
			}()
			break
		}
		server.Stop()
	}
	test.That(t, success, test.ShouldBeTrue)

	rc, err := connect(port, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, rc.Close(context.Background()), test.ShouldBeNil)
	}()

	// Gizmo is a custom component model and API.
	t.Run("Test Gizmo", func(t *testing.T) {
		res, err := rc.ResourceByName(gizmoapi.Named("gizmo1"))
		test.That(t, err, test.ShouldBeNil)

		giz := res.(gizmoapi.Gizmo)
		ret1, err := giz.DoOne(context.Background(), "1.0")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret1, test.ShouldBeTrue)

		// also tests that the ForeignServiceHandler does not drop the first message
		ret2, err := giz.DoOneClientStream(context.Background(), []string{"1.0", "2.0", "3.0"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret2, test.ShouldBeFalse)

		ret2, err = giz.DoOneClientStream(context.Background(), []string{"0", "2.0", "3.0"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret2, test.ShouldBeTrue)

		ret3, err := giz.DoOneServerStream(context.Background(), "1.0")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret3, test.ShouldResemble, []bool{true, false, true, false})

		ret3, err = giz.DoOneBiDiStream(context.Background(), []string{"1.0", "2.0", "3.0"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret3, test.ShouldResemble, []bool{true, true, true})

		ret4, err := giz.DoTwo(context.Background(), true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret4, test.ShouldEqual, "sum=4")

		ret4, err = giz.DoTwo(context.Background(), false)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret4, test.ShouldEqual, "sum=5")
	})

	// Summation is a custom service model and API.
	t.Run("Test Summation", func(t *testing.T) {
		res, err := rc.ResourceByName(summationapi.Named("adder"))
		test.That(t, err, test.ShouldBeNil)
		add := res.(summationapi.Summation)
		nums := []float64{10, 0.5, 12}
		retAdd, err := add.Sum(context.Background(), nums)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, retAdd, test.ShouldEqual, 22.5)
	})
}

func connect(port int, logger logging.Logger) (robot.Robot, error) {
	connectCtx, cancelConn := context.WithTimeout(context.Background(), time.Second*30)
	defer cancelConn()
	for {
		dialCtx, dialCancel := context.WithTimeout(context.Background(), time.Millisecond*500)
		rc, err := client.New(dialCtx, fmt.Sprintf("localhost:%d", port), logger,
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

func modifyCfg(t *testing.T, cfgIn string, logger logging.Logger) (string, int, error) {
	gizmoModPath := testutils.BuildTempModule(t, "examples/customresources/demos/multiplemodules/gizmomodule")
	summationModPath := testutils.BuildTempModule(t, "examples/customresources/demos/multiplemodules/summationmodule")

	port, err := goutils.TryReserveRandomPort()
	if err != nil {
		return "", 0, err
	}

	cfg, err := config.Read(context.Background(), cfgIn, logger)
	if err != nil {
		return "", 0, err
	}
	cfg.Network.BindAddress = fmt.Sprintf("localhost:%d", port)
	cfg.Modules[0].ExePath = gizmoModPath
	cfg.Modules[1].ExePath = summationModPath
	output, err := json.Marshal(cfg)
	if err != nil {
		return "", 0, err
	}
	file, err := os.CreateTemp(t.TempDir(), "viam-test-config-*")
	if err != nil {
		return "", 0, err
	}
	cfgFilename := file.Name()
	_, err = file.Write(output)
	if err != nil {
		return "", 0, err
	}
	return cfgFilename, port, file.Close()
}
