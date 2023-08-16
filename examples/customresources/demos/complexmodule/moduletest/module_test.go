// Package main tests out all four custom models in the complexmodule.
package main_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/utils"
)

// This test ends up being a great validation of the logical clock on resource graph node
// modifications since the base depends on something it needs during initialization that
// needs to be added to the web service before it normally would be avalilable after completing
// a config cycle.
func TestComplexModule(t *testing.T) {
	logger := golog.NewTestLogger(t)

	// Modify the example config to run directly, without compiling the module first.
	cfgFilename, port, err := modifyCfg(t, utils.ResolveFile("examples/customresources/demos/complexmodule/module.json"), logger)
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

	// Gizmo is a custom component model and API.
	t.Run("Test Gizmo", func(t *testing.T) {
		res, err := rc.ResourceByName(gizmoapi.Named("gizmo1"))
		test.That(t, err, test.ShouldBeNil)

		giz := res.(gizmoapi.Gizmo)
		ret1, err := giz.DoOne(context.Background(), "hello")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret1, test.ShouldBeFalse)

		ret2, err := giz.DoOneClientStream(context.Background(), []string{"hello", "arg1", "foo"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret2, test.ShouldBeFalse)

		ret2, err = giz.DoOneClientStream(context.Background(), []string{"arg1", "arg1", "arg1"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret2, test.ShouldBeTrue)

		ret3, err := giz.DoOneServerStream(context.Background(), "hello")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret3, test.ShouldResemble, []bool{false, false, true, false})

		ret3, err = giz.DoOneBiDiStream(context.Background(), []string{"hello", "arg1", "foo"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret3, test.ShouldResemble, []bool{true, false})

		ret3, err = giz.DoOneBiDiStream(context.Background(), []string{"arg1", "arg1", "arg1"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret3, test.ShouldResemble, []bool{true, true})
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

		res, err = rc.ResourceByName(summationapi.Named("subtractor"))
		test.That(t, err, test.ShouldBeNil)

		sub := res.(summationapi.Summation)
		retSub, err := sub.Sum(context.Background(), nums)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, retSub, test.ShouldEqual, -22.5)
	})

	// Base is a custom component, but built-in API. It also depends on built-in motors, so tests dependencies.
	t.Run("Test Base", func(t *testing.T) {
        res, err := rc.ResourceByName(motor.Named("motor1"))
		test.That(t, err, test.ShouldBeNil)
		motorL := res.(motor.Motor)

		res, err = rc.ResourceByName(motor.Named("motor2"))
		test.That(t, err, test.ShouldBeNil)
		motorR := res.(motor.Motor)

		res, err = rc.ResourceByName(base.Named("base1"))
		test.That(t, err, test.ShouldBeNil)
		mybase := res.(base.Base)

		// Test generic echo
		testCmd := map[string]interface{}{"foo": "bar"}
		ret, err := mybase.DoCommand(context.Background(), testCmd)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret, test.ShouldResemble, testCmd)

		// Stopped to begin with
		moving, speed, err := motorL.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, speed, test.ShouldEqual, 0.0)

		moving, speed, err = motorR.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, speed, test.ShouldEqual, 0.0)

		// Forward
		err = mybase.SetPower(context.Background(), r3.Vector{Y: 1}, r3.Vector{}, nil)
		test.That(t, err, test.ShouldBeNil)

		moving, speed, err = motorL.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeTrue)
		test.That(t, speed, test.ShouldEqual, 1.0)

		moving, speed, err = motorR.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeTrue)
		test.That(t, speed, test.ShouldEqual, 1.0)

		// Stop
		err = mybase.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		moving, speed, err = motorL.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, speed, test.ShouldEqual, 0.0)

		moving, speed, err = motorR.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, speed, test.ShouldEqual, 0.0)

		// Backward
		err = mybase.SetPower(context.Background(), r3.Vector{Y: -1}, r3.Vector{}, nil)
		test.That(t, err, test.ShouldBeNil)

		moving, speed, err = motorL.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeTrue)
		test.That(t, speed, test.ShouldEqual, -1.0)

		moving, speed, err = motorR.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeTrue)
		test.That(t, speed, test.ShouldEqual, -1.0)

		// Stop
		err = mybase.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		moving, speed, err = motorL.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, speed, test.ShouldEqual, 0.0)

		moving, speed, err = motorR.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, speed, test.ShouldEqual, 0.0)

		// Spin Left
		err = mybase.SetPower(context.Background(), r3.Vector{}, r3.Vector{Z: 1}, nil)
		test.That(t, err, test.ShouldBeNil)

		moving, speed, err = motorL.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeTrue)
		test.That(t, speed, test.ShouldEqual, -1.0)

		moving, speed, err = motorR.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeTrue)
		test.That(t, speed, test.ShouldEqual, 1.0)

		// Stop
		err = mybase.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		moving, speed, err = motorL.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, speed, test.ShouldEqual, 0.0)

		moving, speed, err = motorR.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, speed, test.ShouldEqual, 0.0)

		// Spin Right
		err = mybase.SetPower(context.Background(), r3.Vector{}, r3.Vector{Z: -1}, nil)
		test.That(t, err, test.ShouldBeNil)

		moving, speed, err = motorL.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeTrue)
		test.That(t, speed, test.ShouldEqual, 1.0)

		moving, speed, err = motorR.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeTrue)
		test.That(t, speed, test.ShouldEqual, -1.0)

		// Stop
		err = mybase.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		moving, speed, err = motorL.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, speed, test.ShouldEqual, 0.0)

		moving, speed, err = motorR.IsPowered(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, moving, test.ShouldBeFalse)
		test.That(t, speed, test.ShouldEqual, 0.0)
	})

	// Navigation is a custom model, but built-in API.
	t.Run("Test Navigation", func(t *testing.T) {
		res, err := rc.ResourceByName(navigation.Named("denali"))
		test.That(t, err, test.ShouldBeNil)

		nav := res.(navigation.Service)
		geoPose, err := nav.Location(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, geoPose.Location().Lat(), test.ShouldAlmostEqual, 63.0691739667009)
		test.That(t, geoPose.Location().Lng(), test.ShouldAlmostEqual, -151.00698515692034)

		err = nav.AddWaypoint(context.Background(), geo.NewPoint(55.1, 22.2), nil)
		test.That(t, err, test.ShouldBeNil)

		err = nav.AddWaypoint(context.Background(), geo.NewPoint(10.77, 17.88), nil)
		test.That(t, err, test.ShouldBeNil)

		err = nav.AddWaypoint(context.Background(), geo.NewPoint(42.0, 42.0), nil)
		test.That(t, err, test.ShouldBeNil)

		waypoints, err := nav.Waypoints(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		expected := []navigation.Waypoint{
			{Lat: 55.1, Long: 22.2},
			{Lat: 10.77, Long: 17.88},
			{Lat: 42.0, Long: 42.0},
		}

		test.That(t, waypoints, test.ShouldResemble, expected)
	})
}

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

func modifyCfg(t *testing.T, cfgIn string, logger golog.Logger) (string, string, error) {
	modPath, err := testutils.BuildTempModule(t, "examples/customresources/demos/complexmodule")
	if err != nil {
		return "", "", err
	}

	p, err := goutils.TryReserveRandomPort()
	if err != nil {
		return "", "", err
	}
	port := strconv.Itoa(p)

	// workaround because config.Read can't validate a module config with a "missing" ExePath
	touchFile("./complexmodule")
	defer os.Remove("./complexmodule")
	cfg, err := config.Read(context.Background(), cfgIn, logger)
	if err != nil {
		return "", "", err
	}
	cfg.Network.BindAddress = "localhost:" + port
	cfg.Modules[0].ExePath = modPath
	output, err := json.Marshal(cfg)
	if err != nil {
		return "", "", err
	}
	file, err := os.CreateTemp(t.TempDir(), "viam-test-config-*")
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

func touchFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return f.Close()
}

func TestValidationFailure(t *testing.T) {
	logger, logs := golog.NewObservedTestLogger(t)

	// bad_modular_validation.json contains a "mybase" modular component that will
	// fail modular Validation due to a missing "motorL" attribute.
	cfgFilename, port, err := modifyCfg(t,
		utils.ResolveFile("examples/customresources/demos/complexmodule/moduletest/bad_modular_validation.json"), logger)
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

	// Assert that motors were added but base was not.
	_, err = rc.ResourceByName(motor.Named("motor1"))
	test.That(t, err, test.ShouldBeNil)
	_, err = rc.ResourceByName(motor.Named("motor2"))
	test.That(t, err, test.ShouldBeNil)
	_, err = rc.ResourceByName(base.Named("base1"))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldResemble, `resource "rdk:component:base/base1" not found`)

	// Assert that Validation failure is present in server output, but build failure
	// is not.
	test.That(t, logs.FilterMessageSnippet(
		"config validation error found in resource: base1").Len(), test.ShouldEqual, 1)
	test.That(t, logs.FilterMessageSnippet("error building component").Len(), test.ShouldEqual, 0)
}
