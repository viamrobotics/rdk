package tpspace

import (
	"math"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

func TestAlphaIdx(t *testing.T) {
	for i := uint(0); i < defaultAlphaCnt; i++ {
		alpha := index2alpha(i, defaultAlphaCnt)
		i2 := alpha2index(alpha, defaultAlphaCnt)
		test.That(t, i, test.ShouldEqual, i2)
	}
}

func alpha2index(alpha float64, numPaths uint) uint {
	alpha = wrapTo2Pi(alpha+math.Pi) - math.Pi
	idx := uint(math.Round(0.5 * (float64(numPaths)*(1.0+alpha/math.Pi) - 1.0)))
	return idx
}

func TestPtgDiffDrive(t *testing.T) {
	p := NewDiffDrivePTG(0)
	pose, err := p.Transform([]referenceframe.Input{{-3.0}, {10}})
	test.That(t, err, test.ShouldBeNil)
	goalPose := spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -10})
	test.That(t, spatialmath.PoseAlmostEqual(pose, goalPose), test.ShouldBeTrue)
}

func TestPtgTransform(t *testing.T) {
	pFrame, err := NewPTGFrameFromKinematicOptions(
		"",
		logging.NewTestLogger(t),
		1.,
		2,
		nil,
		true,
		true,
	)
	test.That(t, err, test.ShouldBeNil)
	p, ok := pFrame.(*ptgGroupFrame)
	test.That(t, ok, test.ShouldBeTrue)
	pose, err := p.Transform([]referenceframe.Input{{0}, {math.Pi / 2}, {0}, {200}})
	test.That(t, err, test.ShouldBeNil)
	traj, err := p.PTGSolvers()[0].Trajectory(math.Pi/2, 0, 200, defaultResolution)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostEqual(pose, traj[len(traj)-1].Pose), test.ShouldBeTrue)
	test.That(t, spatialmath.PoseAlmostEqual(spatialmath.NewZeroPose(), traj[0].Pose), test.ShouldBeTrue)

	poseInv, err := p.Transform([]referenceframe.Input{{0}, {math.Pi / 2}, {200}, {0}})
	test.That(t, err, test.ShouldBeNil)
	trajInv, err := p.PTGSolvers()[0].Trajectory(math.Pi/2, 200, 0, defaultResolution)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, spatialmath.PoseAlmostEqual(poseInv, trajInv[len(trajInv)-1].Pose), test.ShouldBeTrue)
}
