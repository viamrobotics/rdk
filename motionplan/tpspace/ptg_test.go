package tpspace

import (
	"math"
	"testing"

	"go.viam.com/test"

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
