package modmanager

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	goutils "go.viam.com/utils"

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

	t.Log("test AddModuleHelpers")
	mgr, err := NewManager(myRobot)
	test.That(t, err, test.ShouldBeNil)

	mod := &module{name: "test", exe: modExe}

	err = mod.startProcess(ctx, parentAddr, logger)
	test.That(t, err, test.ShouldBeNil)

	err = mod.dial()
	test.That(t, err, test.ShouldBeNil)

	err = mod.checkReady(ctx, parentAddr)
	test.That(t, err, test.ShouldBeNil)

	mod.registerResources(mgr, logger)
	reg := registry.ComponentLookup(generic.Subtype, myCounterModel)
	test.That(t, reg, test.ShouldNotBeNil)
	test.That(t, reg.Constructor, test.ShouldNotBeNil)

	test.That(t, mgr.Close(ctx), test.ShouldBeNil)
	test.That(t, mod.process.Stop(), test.ShouldBeNil)

	registry.DeregisterComponent(generic.Subtype, myCounterModel)

	reg = registry.ComponentLookup(generic.Subtype, myCounterModel)
	test.That(t, reg, test.ShouldBeNil)

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

	err = goutils.TryClose(ctx, counter)
	test.That(t, err, test.ShouldBeNil)

	err = mgr.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}
