package motionplan

import (
	"testing"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/spatialmath"
)

func TestFixOvIncrement(t *testing.T) {
	pos1 := commonpb.Pose{
		X:     -66,
		Y:     -133,
		Z:     372,
		Theta: 15,
		OX:    0,
		OY:    1,
		OZ:    0,
	}
	pos2 := pos1

	// Increment, but we're not pointing at Z axis, so should do nothing
	pos2.OX = -0.1
	outpos := fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos, test.ShouldResemble, spatialmath.NewPoseFromProtobuf(&pos2))

	// point at positive Z axis, decrement OX, should subtract 180
	pos1.OZ = 1
	pos2.OZ = 1
	pos1.OY = 0
	pos2.OY = 0
	outpos = fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos.Orientation().OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, -165)

	// Spatial translation is incremented, should do nothing
	pos2.X -= 0.1
	outpos = fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos, test.ShouldResemble, spatialmath.NewPoseFromProtobuf(&pos2))

	// Point at -Z, increment OY
	pos2.X += 0.1
	pos2.OX += 0.1
	pos1.OZ = -1
	pos2.OZ = -1
	pos2.OY = 0.1
	outpos = fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos.Orientation().OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 105)

	// OX and OY are both incremented, should do nothing
	pos2.OX += 0.1
	outpos = fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos, test.ShouldResemble, spatialmath.NewPoseFromProtobuf(&pos2))
}
