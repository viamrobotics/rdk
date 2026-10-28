package builtin

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/motionplan/armplanning"
)

func TestApplyCableConstraintDefaults(t *testing.T) {
	on, off := true, false
	conf := &Config{
		CableConstraint:         &on,
		MaxCableTwistDegs:       90,
		CableTwistReferenceDegs: map[string][]float64{"arm": {0, 0, 0, 0, 0, 45}},
	}
	optsFor := func(c *Config, extra map[string]interface{}) *armplanning.PlannerOptions {
		opts, err := armplanning.NewPlannerOptionsFromExtra(extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, c.applyCableConstraintDefaults(opts, extra), test.ShouldBeNil)
		return opts
	}

	t.Run("config fills options the request leaves unset", func(t *testing.T) {
		opts := optsFor(conf, map[string]interface{}{})
		test.That(t, opts.CableConstraint, test.ShouldBeTrue)
		test.That(t, opts.CableConstraintFrames, test.ShouldBeEmpty)
		test.That(t, opts.MaxCableTwistDegs, test.ShouldEqual, 90)
		test.That(t, opts.CableTwistReferenceDegs, test.ShouldResemble, conf.CableTwistReferenceDegs)
	})

	t.Run("request extra overrides config", func(t *testing.T) {
		opts := optsFor(conf, map[string]interface{}{"cable_constraint": false, "max_cable_twist_degs": 120.})
		test.That(t, opts.CableConstraint, test.ShouldBeFalse)
		test.That(t, opts.MaxCableTwistDegs, test.ShouldEqual, 120)
		test.That(t, opts.CableTwistReferenceDegs, test.ShouldResemble, conf.CableTwistReferenceDegs)
	})

	t.Run("an empty config leaves the constraint off", func(t *testing.T) {
		opts := optsFor(&Config{}, map[string]interface{}{})
		test.That(t, opts.CableConstraint, test.ShouldBeFalse)
		test.That(t, opts.CableConstraintFrames, test.ShouldBeEmpty)
		test.That(t, opts.MaxCableTwistDegs, test.ShouldEqual, 180)
		test.That(t, opts.CableTwistReferenceDegs, test.ShouldBeNil)
	})

	t.Run("config frames alone do not enable the constraint", func(t *testing.T) {
		opts := optsFor(&Config{CableConstraintFrames: []string{"arm2"}}, map[string]interface{}{})
		test.That(t, opts.CableConstraint, test.ShouldBeFalse)
		test.That(t, opts.CableConstraintFrames, test.ShouldResemble, []string{"arm2"})
	})

	t.Run("request extra can opt in without config", func(t *testing.T) {
		opts := optsFor(&Config{}, map[string]interface{}{"cable_constraint": true})
		test.That(t, opts.CableConstraint, test.ShouldBeTrue)
	})

	t.Run("config set to false is overridden by the request", func(t *testing.T) {
		opts := optsFor(&Config{CableConstraint: &off}, map[string]interface{}{"cable_constraint": true})
		test.That(t, opts.CableConstraint, test.ShouldBeTrue)
	})

	t.Run("config frames scope an explicit enable", func(t *testing.T) {
		opts := optsFor(&Config{CableConstraint: &on, CableConstraintFrames: []string{"arm2"}}, map[string]interface{}{})
		test.That(t, opts.CableConstraint, test.ShouldBeTrue)
		test.That(t, opts.CableConstraintFrames, test.ShouldResemble, []string{"arm2"})
	})

	t.Run("a non-positive max twist fails once the constraint is enabled", func(t *testing.T) {
		extra := map[string]interface{}{"max_cable_twist_degs": 0.}
		opts, err := armplanning.NewPlannerOptionsFromExtra(extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, (&Config{CableConstraint: &on}).applyCableConstraintDefaults(opts, extra), test.ShouldNotBeNil)
	})

	t.Run("negative max twist fails validation", func(t *testing.T) {
		_, _, err := (&Config{MaxCableTwistDegs: -1}).Validate("")
		test.That(t, err, test.ShouldNotBeNil)
	})
}
