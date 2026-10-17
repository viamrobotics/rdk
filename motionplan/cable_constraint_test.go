package motionplan

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

func TestCableConstraint(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	fs := referenceframe.NewEmptyFrameSystem("test")
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fs.AddFrame(m, fs.World()), test.ShouldBeNil)

	cfg := func(joint int, degs float64) *referenceframe.LinearInputs {
		in := make([]referenceframe.Input, 6)
		in[joint] = utils.DegToRad(degs)
		return referenceframe.FrameSystemInputs{m.Name(): in}.ToLinearInputs()
	}

	checkerFor := func(start *referenceframe.LinearInputs, cc CableConstraint) *ConstraintChecker {
		c := NewEmptyConstraintChecker(logger)
		test.That(t, c.AddCableConstraint(fs, start, cc), test.ShouldBeNil)
		return c
	}
	check := func(c *ConstraintChecker, joint int, degs float64) error {
		_, err := c.CheckStateFSConstraints(ctx, &StateFS{Configuration: cfg(joint, degs), FS: fs})
		return err
	}

	t.Run("default limit is half a turn from zero", func(t *testing.T) {
		c := checkerFor(cfg(0, 0), CableConstraint{})
		test.That(t, check(c, 5, 170), test.ShouldBeNil)
		test.That(t, check(c, 5, -170), test.ShouldBeNil)
		test.That(t, errors.Is(check(c, 5, 190), ErrCableConstraintViolated), test.ShouldBeTrue)
		test.That(t, errors.Is(check(c, 0, -190), ErrCableConstraintViolated), test.ShouldBeTrue)
		err := c.CheckStateFSTopoOnly(&StateFS{Configuration: cfg(5, 190), FS: fs})
		test.That(t, errors.Is(err, ErrCableConstraintViolated), test.ShouldBeTrue)
	})

	t.Run("custom limit and reference", func(t *testing.T) {
		c := checkerFor(cfg(5, 90), CableConstraint{
			MaxTwistDegs:  90,
			ReferenceDegs: map[string][]float64{m.Name(): {0, 0, 0, 0, 0, 90}},
		})
		test.That(t, check(c, 5, 170), test.ShouldBeNil)
		test.That(t, check(c, 5, 10), test.ShouldBeNil)
		test.That(t, errors.Is(check(c, 5, -10), ErrCableConstraintViolated), test.ShouldBeTrue)
		test.That(t, errors.Is(check(c, 5, 190), ErrCableConstraintViolated), test.ShouldBeTrue)
	})

	t.Run("a start beyond the limit may unwind but not wind further", func(t *testing.T) {
		c := checkerFor(cfg(5, 300), CableConstraint{})
		test.That(t, check(c, 5, 300), test.ShouldBeNil)
		test.That(t, check(c, 5, 250), test.ShouldBeNil)
		test.That(t, errors.Is(check(c, 5, 310), ErrCableConstraintViolated), test.ShouldBeTrue)
		test.That(t, errors.Is(check(c, 5, -190), ErrCableConstraintViolated), test.ShouldBeTrue)
	})

	t.Run("invalid references", func(t *testing.T) {
		c := NewEmptyConstraintChecker(logger)
		err := c.AddCableConstraint(fs, cfg(0, 0), CableConstraint{ReferenceDegs: map[string][]float64{m.Name(): {0}}})
		test.That(t, err, test.ShouldNotBeNil)
		err = c.AddCableConstraint(fs, cfg(0, 0), CableConstraint{ReferenceDegs: map[string][]float64{"nope": {0}}})
		test.That(t, err, test.ShouldNotBeNil)
		err = c.AddCableConstraint(fs, cfg(0, 0), CableConstraint{Frames: []string{"nope"}})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("frames limit which models are checked", func(t *testing.T) {
		c := checkerFor(cfg(0, 0), CableConstraint{Frames: []string{m.Name()}})
		test.That(t, errors.Is(check(c, 5, 190), ErrCableConstraintViolated), test.ShouldBeTrue)
	})
}
