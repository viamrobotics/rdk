package builtin

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func TestApplyCableConstraintDefaults(t *testing.T) {
	on, off := true, false
	conf := &Config{
		CableConstraint:         &on,
		MaxCableTwistDegs:       90,
		CableTwistReferenceDegs: map[string][]float64{"arm": {0, 0, 0, 0, 0, 45}},
	}
	optsFor := func(c *Config, extra map[string]interface{}, cameraArms []string) *armplanning.PlannerOptions {
		opts, err := armplanning.NewPlannerOptionsFromExtra(extra)
		test.That(t, err, test.ShouldBeNil)
		c.applyCableConstraintDefaults(opts, extra, cameraArms)
		return opts
	}

	t.Run("config fills options the request leaves unset", func(t *testing.T) {
		opts := optsFor(conf, map[string]interface{}{}, nil)
		test.That(t, opts.CableConstraint, test.ShouldBeTrue)
		test.That(t, opts.CableConstraintFrames, test.ShouldBeEmpty)
		test.That(t, opts.MaxCableTwistDegs, test.ShouldEqual, 90)
		test.That(t, opts.CableTwistReferenceDegs, test.ShouldResemble, conf.CableTwistReferenceDegs)
	})

	t.Run("request extra overrides config", func(t *testing.T) {
		opts := optsFor(conf, map[string]interface{}{"cable_constraint": false, "max_cable_twist_degs": 120.}, nil)
		test.That(t, opts.CableConstraint, test.ShouldBeFalse)
		test.That(t, opts.MaxCableTwistDegs, test.ShouldEqual, 120)
		test.That(t, opts.CableTwistReferenceDegs, test.ShouldResemble, conf.CableTwistReferenceDegs)
	})

	t.Run("an empty config with no cameras on arms keeps planner defaults", func(t *testing.T) {
		opts := optsFor(&Config{}, map[string]interface{}{}, nil)
		test.That(t, opts.CableConstraint, test.ShouldBeFalse)
		test.That(t, opts.MaxCableTwistDegs, test.ShouldEqual, 180)
		test.That(t, opts.CableTwistReferenceDegs, test.ShouldBeNil)
	})

	t.Run("an unset config guesses from cameras on arms", func(t *testing.T) {
		opts := optsFor(&Config{}, map[string]interface{}{}, []string{"arm1"})
		test.That(t, opts.CableConstraint, test.ShouldBeTrue)
		test.That(t, opts.CableConstraintFrames, test.ShouldResemble, []string{"arm1"})
	})

	t.Run("config or request set to false disables the guess", func(t *testing.T) {
		opts := optsFor(&Config{CableConstraint: &off}, map[string]interface{}{}, []string{"arm1"})
		test.That(t, opts.CableConstraint, test.ShouldBeFalse)
		opts = optsFor(&Config{}, map[string]interface{}{"cable_constraint": false}, []string{"arm1"})
		test.That(t, opts.CableConstraint, test.ShouldBeFalse)
	})

	t.Run("negative max twist fails validation", func(t *testing.T) {
		_, _, err := (&Config{MaxCableTwistDegs: -1}).Validate("")
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestArmsCarryingCameras(t *testing.T) {
	fs := referenceframe.NewEmptyFrameSystem("test")
	for _, name := range []string{"arm1", "arm2"} {
		m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), name)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, fs.AddFrame(m, fs.World()), test.ShouldBeNil)
	}
	gripper, err := referenceframe.NewStaticFrame("gripper", spatialmath.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(gripper, fs.Frame("arm1")), test.ShouldBeNil)
	wristCam, err := referenceframe.NewStaticFrame("wrist-cam", spatialmath.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(wristCam, gripper), test.ShouldBeNil)
	fixedCam, err := referenceframe.NewStaticFrame("fixed-cam", spatialmath.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(fixedCam, fs.World()), test.ShouldBeNil)

	components := map[string]resource.Resource{
		"arm1":      inject.NewArm("arm1"),
		"arm2":      inject.NewArm("arm2"),
		"wrist-cam": inject.NewCamera("wrist-cam"),
		"fixed-cam": inject.NewCamera("fixed-cam"),
	}
	test.That(t, armsCarryingCameras(fs, components), test.ShouldResemble, []string{"arm1"})

	delete(components, "wrist-cam")
	test.That(t, armsCarryingCameras(fs, components), test.ShouldBeEmpty)
}
