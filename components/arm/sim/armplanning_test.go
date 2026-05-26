//go:build !windows && !no_cgo

// TestMoveToPosition exercises the integration with go.viam.com/rdk/motionplan/armplanning,
// which itself depends on nlopt for inverse kinematics. nlopt is cgo-required, so this
// test only runs under the cgo build.

package sim

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

func TestMoveToPosition(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	resConf := resource.Config{
		Name:  "arm",
		API:   arm.API,
		Model: Model,
		ConvertedAttributes: &Config{
			Model:        "ur5e",
			Speed:        100.0,
			SimulateTime: true,
		},
	}

	simArmI, err := NewArm(ctx, nil, resConf, logger)
	test.That(t, err, test.ShouldBeNil)
	simArm := simArmI.(*simulatedArm)
	defer func() {
		err = simArm.Close(ctx)
		test.That(t, err, test.ShouldBeNil)
	}()

	startPose, err := simArm.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	// Nudge the end-effector ~50mm along x. MoveToPosition plans + executes through joint positions.
	target := spatialmath.NewPose(
		r3.Vector{
			X: startPose.Point().X + 50,
			Y: startPose.Point().Y,
			Z: startPose.Point().Z,
		},
		startPose.Orientation(),
	)
	err = simArm.MoveToPosition(ctx, target, nil)
	test.That(t, err, test.ShouldBeNil)

	endPose, err := simArm.EndPosition(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	// Planner tolerance is millimeter-scale; allow a small slop.
	test.That(t, spatialmath.PoseAlmostCoincidentEps(endPose, target, 5.0), test.ShouldBeTrue)
}
