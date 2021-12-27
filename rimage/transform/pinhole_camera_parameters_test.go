package transform

import (
	"testing"

	"go.viam.com/test"
)

func TestDepthColorIntrinsicsExtrinsics(t *testing.T) {
	jsonFilePath := "../../robots/configs/intel515_parameters.json"

	// check depth sensor parameters values
	depthIntrinsics, err := NewPinholeCameraIntrinsicsFromJSONFile(jsonFilePath, "depth")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, depthIntrinsics.Height, test.ShouldEqual, 768)
	test.That(t, depthIntrinsics.Width, test.ShouldEqual, 1024)
	test.That(t, depthIntrinsics.Fx, test.ShouldEqual, 734.938)
	test.That(t, depthIntrinsics.Fy, test.ShouldEqual, 735.516)

	// check color sensor parameters values
	colorIntrinsics, err2 := NewPinholeCameraIntrinsicsFromJSONFile(jsonFilePath, "color")
	test.That(t, err2, test.ShouldBeNil)
	test.That(t, colorIntrinsics.Height, test.ShouldEqual, 720)
	test.That(t, colorIntrinsics.Width, test.ShouldEqual, 1280)
	test.That(t, colorIntrinsics.Fx, test.ShouldEqual, 900.538)
	test.That(t, colorIntrinsics.Fy, test.ShouldEqual, 900.818)

	// check sensorParams sensor parameters values
	sensorParams, err3 := NewDepthColorIntrinsicsExtrinsicsFromJSONFile(jsonFilePath)
	test.That(t, err3, test.ShouldBeNil)
	gtRotation := []float64{0.999958, -0.00838489, 0.00378392, 0.00824708, 0.999351, 0.0350734, -0.00407554, -0.0350407, 0.999378}

	test.That(t, sensorParams.ExtrinsicD2C.RotationMatrix, test.ShouldHaveLength, 9)
	test.That(t, sensorParams.ExtrinsicD2C.RotationMatrix, test.ShouldResemble, gtRotation)
	test.That(t, sensorParams.ExtrinsicD2C.TranslationVector, test.ShouldHaveLength, 3)
}

func TestTransformPointToPoint(t *testing.T) {
	x1, y1, z1 := 0., 0., 1.
	rot1 := []float64{1, 0, 0, 0, 1, 0, 0, 0, 1}

	t1 := []float64{0, 0, 1}
	// Get rigid body transform between Depth and RGB sensor
	extrinsics1 := Extrinsics{
		RotationMatrix:    rot1,
		TranslationVector: t1,
	}
	x2, y2, z2 := extrinsics1.TransformPointToPoint(x1, y1, z1)
	test.That(t, x2, test.ShouldEqual, 0.)
	test.That(t, y2, test.ShouldEqual, 0.)
	test.That(t, z2, test.ShouldEqual, 2.)

	t2 := []float64{0, 2, 0}
	extrinsics2 := Extrinsics{
		RotationMatrix:    rot1,
		TranslationVector: t2,
	}
	x3, y3, z3 := extrinsics2.TransformPointToPoint(x1, y1, z1)
	test.That(t, x3, test.ShouldEqual, 0.)
	test.That(t, y3, test.ShouldEqual, 2.)
	test.That(t, z3, test.ShouldEqual, 1.)
	// Rotation in the (z,x) plane of 90 degrees
	rot2 := []float64{0, 0, 1, 0, 1, 0, 0, 0, -1}
	extrinsics3 := Extrinsics{
		RotationMatrix:    rot2,
		TranslationVector: t2,
	}
	x4, y4, z4 := extrinsics3.TransformPointToPoint(x1, y1, z1)
	test.That(t, x4, test.ShouldEqual, 1.)
	test.That(t, y4, test.ShouldEqual, 2.)
	test.That(t, z4, test.ShouldEqual, -1.)
}
