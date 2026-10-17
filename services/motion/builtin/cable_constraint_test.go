package builtin

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/motionplan/armplanning"
)

func TestApplyCableConstraintDefaults(t *testing.T) {
	conf := &Config{
		CableConstraint:         true,
		MaxCableTwistDegs:       90,
		CableTwistReferenceDegs: map[string][]float64{"arm": {0, 0, 0, 0, 0, 45}},
	}

	t.Run("config fills options the request leaves unset", func(t *testing.T) {
		extra := map[string]interface{}{}
		opts, err := armplanning.NewPlannerOptionsFromExtra(extra)
		test.That(t, err, test.ShouldBeNil)
		conf.applyCableConstraintDefaults(opts, extra)
		test.That(t, opts.CableConstraint, test.ShouldBeTrue)
		test.That(t, opts.MaxCableTwistDegs, test.ShouldEqual, 90)
		test.That(t, opts.CableTwistReferenceDegs, test.ShouldResemble, conf.CableTwistReferenceDegs)
	})

	t.Run("request extra overrides config", func(t *testing.T) {
		extra := map[string]interface{}{"cable_constraint": false, "max_cable_twist_degs": 120.}
		opts, err := armplanning.NewPlannerOptionsFromExtra(extra)
		test.That(t, err, test.ShouldBeNil)
		conf.applyCableConstraintDefaults(opts, extra)
		test.That(t, opts.CableConstraint, test.ShouldBeFalse)
		test.That(t, opts.MaxCableTwistDegs, test.ShouldEqual, 120)
		test.That(t, opts.CableTwistReferenceDegs, test.ShouldResemble, conf.CableTwistReferenceDegs)
	})

	t.Run("an empty config keeps planner defaults", func(t *testing.T) {
		extra := map[string]interface{}{}
		opts, err := armplanning.NewPlannerOptionsFromExtra(extra)
		test.That(t, err, test.ShouldBeNil)
		(&Config{}).applyCableConstraintDefaults(opts, extra)
		test.That(t, opts.CableConstraint, test.ShouldBeFalse)
		test.That(t, opts.MaxCableTwistDegs, test.ShouldEqual, 180)
		test.That(t, opts.CableTwistReferenceDegs, test.ShouldBeNil)
	})

	t.Run("negative max twist fails validation", func(t *testing.T) {
		_, _, err := (&Config{MaxCableTwistDegs: -1}).Validate("")
		test.That(t, err, test.ShouldNotBeNil)
	})
}
