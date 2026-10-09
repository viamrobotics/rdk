package robotimpl

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/robottestutils"
	"go.viam.com/rdk/utils"
)

// Repro for https://github.com/viamrobotics/rdk/issues/6627, end to end over gRPC with a fake
// xArm6 whose tool points a fraction of a degree off straight down. The truth is this process's
// own frame system, which never goes through an orientation vector; the client only sees poses
// through the API. Each case has a 2 degree control, outside the affected zone.
func TestNearVerticalArmPosesOverAPI(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg, err := config.FromReader(ctx, "", strings.NewReader(`{"components": [{
		"name": "arm1", "api": "rdk:component:arm", "model": "rdk:builtin:fake",
		"attributes": {"arm-model": "xarm6"}, "frame": {"parent": "world"}}]}`), logger, nil)
	test.That(t, err, test.ShouldBeNil)
	r := setupLocalRobot(t, ctx, cfg, logger)
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, r.StartWeb(ctx, options), test.ShouldBeNil)
	c := robottestutils.NewRobotClient(t, logger, addr, time.Second)
	remoteArm, err := arm.FromProvider(c, "arm1")
	test.That(t, err, test.ShouldBeNil)

	truth := func(t *testing.T) spatialmath.Pose {
		p, err := r.GetPose(ctx, "arm1", referenceframe.World, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		return p.Pose()
	}
	angleDeg := func(a, b spatialmath.Pose) float64 {
		return utils.RadToDeg(math.Abs(spatialmath.OrientationBetween(a.Orientation(), b.Orientation()).AxisAngles().Theta))
	}

	for _, tiltDeg := range []float64{0.6, 2} {
		// Commands: the server decodes the target from the client's orientation vector.
		t.Run(fmt.Sprintf("MoveToPosition, tool tilted %g deg from straight down toward -X", tiltDeg), func(t *testing.T) {
			// Rz(180) Ry(180 - tilt): the tool's +z axis leans tilt degrees from -Z toward -X.
			target := spatialmath.NewPose(r3.Vector{X: 350, Y: 0, Z: 250}, spatialmath.Compose(
				spatialmath.NewPoseFromOrientation(&spatialmath.R4AA{Theta: math.Pi, RZ: 1}),
				spatialmath.NewPoseFromOrientation(&spatialmath.R4AA{Theta: math.Pi - utils.DegToRad(tiltDeg), RY: 1}),
			).Orientation())
			test.That(t, remoteArm.MoveToPosition(ctx, target, nil), test.ShouldBeNil)
			reached := truth(t)
			test.That(t, reached.Point().Distance(target.Point()), test.ShouldBeLessThan, 0.1)
			test.That(t, angleDeg(reached, target), test.ShouldBeLessThan, 0.01)
		})

		// Reads: the client decodes the server's orientation vector.
		t.Run(fmt.Sprintf("GetPose and EndPosition, tool tilted %g deg from straight down toward -X", tiltDeg), func(t *testing.T) {
			// xarm6 at zero joints points the tool straight down; joint 5 tilts it toward -X.
			joints := []referenceframe.Input{0, 0, 0, 0, utils.DegToRad(tiltDeg), 0}
			test.That(t, remoteArm.MoveToJointPositions(ctx, joints, nil), test.ShouldBeNil)
			want := truth(t)
			viaAPI, err := c.GetPose(ctx, "arm1", referenceframe.World, nil, nil)
			test.That(t, err, test.ShouldBeNil)
			end, err := remoteArm.EndPosition(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			t.Logf("GetPose off by %.3f deg, EndPosition off by %.3f deg", angleDeg(viaAPI.Pose(), want), angleDeg(end, want))
			test.That(t, angleDeg(viaAPI.Pose(), want), test.ShouldBeLessThan, 0.01)
			test.That(t, angleDeg(end, want), test.ShouldBeLessThan, 0.01)
		})
	}
}
