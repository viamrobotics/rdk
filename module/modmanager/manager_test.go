package modmanager

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func TestModManagerFunctions(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	modExe := utils.ResolveFile("examples/customresources/demos/simplemodule/run.sh")

	// Precompile module to avoid timeout issues when building takes too long.
	builder := exec.Command("go", "build", ".")
	builder.Dir = utils.ResolveFile("examples/customresources/demos/simplemodule")
	out, err := builder.CombinedOutput()
	test.That(t, string(out), test.ShouldEqual, "")
	test.That(t, err, test.ShouldBeNil)

	myCounterModel := resource.NewModel("acme", "demo", "mycounter")
	rNameCounter1 := resource.NameFromSubtype(generic.Subtype, "counter1")
	cfgCounter1 := config.Component{
		Name:  "counter1",
		API:   generic.Subtype,
		Model: myCounterModel,
	}
	_, err = cfgCounter1.Validate("test")
	test.That(t, err, test.ShouldBeNil)

	myRobot := &inject.Robot{}
	myRobot.LoggerFunc = func() golog.Logger {
		return logger
	}

	// This cannot use t.TempDir() as the path it gives on MacOS exceeds module.MaxSocketAddressLength.
	parentAddr, err := os.MkdirTemp("", "viam-test-*")
	test.That(t, err, test.ShouldBeNil)
	defer os.RemoveAll(parentAddr)
	parentAddr += "/parent.sock"

	myRobot.ModuleAddressFunc = func() (string, error) {
		return parentAddr, nil
	}

	t.Log("test Helpers")
	mgr, err := NewManager(myRobot)
	test.That(t, err, test.ShouldBeNil)

	mod := &module{name: "test", exe: modExe}

	err = mod.startProcess(ctx, parentAddr, logger)
	test.That(t, err, test.ShouldBeNil)

	err = mod.dial(nil)
	test.That(t, err, test.ShouldBeNil)

	// check that dial can re-use connections.
	oldConn := mod.conn
	err = mod.dial(mod.conn)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mod.conn, test.ShouldEqual, oldConn)

	err = mod.checkReady(ctx, parentAddr)
	test.That(t, err, test.ShouldBeNil)

	mod.registerResources(mgr, logger)
	reg := registry.ComponentLookup(generic.Subtype, myCounterModel)
	test.That(t, reg, test.ShouldNotBeNil)
	test.That(t, reg.Constructor, test.ShouldNotBeNil)

	err = mod.deregisterResources()
	test.That(t, err, test.ShouldBeNil)
	reg = registry.ComponentLookup(generic.Subtype, myCounterModel)
	test.That(t, reg, test.ShouldBeNil)

	test.That(t, mgr.Close(ctx), test.ShouldBeNil)
	test.That(t, mod.process.Stop(), test.ShouldBeNil)

	t.Log("test AddModule")
	mgr, err = NewManager(myRobot)
	test.That(t, err, test.ShouldBeNil)

	modCfg := config.Module{
		Name:    "simple-module",
		ExePath: modExe,
	}
	err = mgr.Add(ctx, modCfg)
	test.That(t, err, test.ShouldBeNil)

	reg = registry.ComponentLookup(generic.Subtype, myCounterModel)
	test.That(t, reg.Constructor, test.ShouldNotBeNil)

	t.Log("test Provides")
	ok := mgr.Provides(cfgCounter1)
	test.That(t, ok, test.ShouldBeTrue)

	cfg2 := config.Component{
		API:   motor.Subtype,
		Model: resource.NewDefaultModel("fake"),
	}
	ok = mgr.Provides(cfg2)
	test.That(t, ok, test.ShouldBeFalse)

	t.Log("test AddResource")
	c, err := mgr.AddResource(ctx, cfgCounter1, nil)
	test.That(t, err, test.ShouldBeNil)
	counter := c.(generic.Generic)

	ret, err := counter.DoCommand(ctx, map[string]interface{}{"command": "get"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret["total"], test.ShouldEqual, 0)

	t.Log("test IsModularResource")
	ok = mgr.IsModularResource(rNameCounter1)
	test.That(t, ok, test.ShouldBeTrue)

	ok = mgr.IsModularResource(resource.NameFromSubtype(generic.Subtype, "missing"))
	test.That(t, ok, test.ShouldBeFalse)

	t.Log("test ValidateConfig")
	// ValidateConfig for cfgCounter1 will not actually call any Validate functionality,
	// as the mycounter model does not have a configuration object with Validate.
	// Assert that ValidateConfig does not fail in this case (allows unimplemented
	// validation).
	deps, err := mgr.ValidateConfig(ctx, cfgCounter1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldBeNil)

	t.Log("test ReconfigureResource")
	// Reconfigure should replace the proxied object, resetting the counter
	ret, err = counter.DoCommand(ctx, map[string]interface{}{"command": "add", "value": 73})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret["total"], test.ShouldEqual, 73)

	err = mgr.ReconfigureResource(ctx, cfgCounter1, nil)
	test.That(t, err, test.ShouldBeNil)

	ret, err = counter.DoCommand(ctx, map[string]interface{}{"command": "get"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret["total"], test.ShouldEqual, 0)

	t.Log("test RemoveResource")
	err = mgr.RemoveResource(ctx, rNameCounter1)
	test.That(t, err, test.ShouldBeNil)

	ok = mgr.IsModularResource(rNameCounter1)
	test.That(t, ok, test.ShouldBeFalse)

	err = mgr.RemoveResource(ctx, rNameCounter1)
	test.That(t, err, test.ShouldNotBeNil)

	_, err = counter.DoCommand(ctx, map[string]interface{}{"command": "get"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no resource with name")

	t.Log("test ReconfigureModule")
	// Re-add counter1.
	_, err = mgr.AddResource(ctx, cfgCounter1, nil)
	test.That(t, err, test.ShouldBeNil)
	// Add 24 to counter and ensure 'total' gets reset after reconfiguration.
	ret, err = counter.DoCommand(ctx, map[string]interface{}{"command": "add", "value": 24})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret["total"], test.ShouldEqual, 24)

	// Change underlying binary path of module to be a copy of simplemodule/run.sh.
	modCfg.ExePath = utils.ResolveFile("module/modmanager/data/simplemoduleruncopy.sh")

	// Reconfigure module with new ExePath.
	orphanedResourceNames, err := mgr.Reconfigure(ctx, modCfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, orphanedResourceNames, test.ShouldBeNil)

	// counter1 should still be provided by reconfigured module.
	ok = mgr.IsModularResource(rNameCounter1)
	test.That(t, ok, test.ShouldBeTrue)
	ret, err = counter.DoCommand(ctx, map[string]interface{}{"command": "get"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret["total"], test.ShouldEqual, 0)

	t.Log("test RemoveModule")
	orphanedResourceNames, err = mgr.Remove("simple-module")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, orphanedResourceNames, test.ShouldResemble, []resource.Name{rNameCounter1})

	ok = mgr.IsModularResource(rNameCounter1)
	test.That(t, ok, test.ShouldBeFalse)
	_, err = counter.DoCommand(ctx, map[string]interface{}{"command": "get"})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "the client connection is closing")

	err = goutils.TryClose(ctx, counter)
	test.That(t, err, test.ShouldBeNil)

	err = mgr.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestModManagerValidation(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	modExe := utils.ResolveFile("examples/customresources/demos/complexmodule/run.sh")

	// Precompile module to avoid timeout issues when building takes too long.
	builder := exec.Command("go", "build", ".")
	builder.Dir = utils.ResolveFile("examples/customresources/demos/complexmodule")
	out, err := builder.CombinedOutput()
	test.That(t, string(out), test.ShouldEqual, "")
	test.That(t, err, test.ShouldBeNil)

	myBaseModel := resource.NewModel("acme", "demo", "mybase")
	cfgMyBase1 := config.Component{
		Name:  "mybase1",
		API:   base.Subtype,
		Model: myBaseModel,
		Attributes: map[string]interface{}{
			"motorL": "motor1",
			"motorR": "motor2",
		},
	}
	_, err = cfgMyBase1.Validate("test")
	test.That(t, err, test.ShouldBeNil)
	// cfgMyBase2 is missing required attributes "motorL" and "motorR" and should
	// cause module Validation error.
	cfgMyBase2 := config.Component{
		Name:  "mybase2",
		API:   base.Subtype,
		Model: myBaseModel,
	}
	_, err = cfgMyBase2.Validate("test")
	test.That(t, err, test.ShouldBeNil)

	myRobot := &inject.Robot{}
	myRobot.LoggerFunc = func() golog.Logger {
		return logger
	}

	// This cannot use t.TempDir() as the path it gives on MacOS exceeds module.MaxSocketAddressLength.
	parentAddr, err := os.MkdirTemp("", "viam-test-*")
	test.That(t, err, test.ShouldBeNil)
	defer os.RemoveAll(parentAddr)
	parentAddr += "/parent.sock"

	myRobot.ModuleAddressFunc = func() (string, error) {
		return parentAddr, nil
	}

	t.Log("adding complex module")
	mgr, err := NewManager(myRobot)
	test.That(t, err, test.ShouldBeNil)

	modCfg := config.Module{
		Name:    "complex-module",
		ExePath: modExe,
	}
	err = mgr.Add(ctx, modCfg)
	test.That(t, err, test.ShouldBeNil)

	reg := registry.ComponentLookup(base.Subtype, myBaseModel)
	test.That(t, reg.Constructor, test.ShouldNotBeNil)

	t.Log("test ValidateConfig")
	deps, err := mgr.ValidateConfig(ctx, cfgMyBase1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldNotBeNil)
	test.That(t, deps[0], test.ShouldResemble, "motor1")
	test.That(t, deps[1], test.ShouldResemble, "motor2")

	_, err = mgr.ValidateConfig(ctx, cfgMyBase2)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldResemble,
		`rpc error: code = Unknown desc = error validating resource: expected "motorL" attribute for mybase "mybase2"`)

	// Test that ValidateConfig respects validateConfigTimeout by artificially
	// lowering it to an impossibly small duration.
	validateConfigTimeout = 1 * time.Nanosecond
	_, err = mgr.ValidateConfig(ctx, cfgMyBase1)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldResemble,
		"rpc error: code = DeadlineExceeded desc = context deadline exceeded")

	err = mgr.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}
