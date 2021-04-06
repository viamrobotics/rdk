package calib

import (
	"testing"
)

func TestDepthColorIntrinsicsExtrinsics(t *testing.T) {
	jsonFilePath := "../../robots/configs/intel515_parameters.json"

	// check depth sensor parameters values
	depthIntrinsics, err := NewPinholeCameraIntrinsicsFromJSONFile(jsonFilePath, "depth")
	if err != nil {
		t.Fatal("Could not read parameters from JSON file.")
	}
	if depthIntrinsics.Height != 768 {
		t.Error("Depth sensor height does not have the right value.")
	}
	if depthIntrinsics.Width != 1024 {
		t.Error("Depth sensor width does not have the right value.")
	}

	if depthIntrinsics.Fx != 734.938 {
		t.Error("Depth sensor focal distance in x does not have the right value.")
	}
	if depthIntrinsics.Fy != 735.516 {
		t.Error("Depth sensor focal distance in y does not have the right value.")
	}

	// check color sensor parameters values
	colorIntrinsics, err2 := NewPinholeCameraIntrinsicsFromJSONFile(jsonFilePath, "color")
	if err2 != nil {
		t.Fatal("Could not read parameters from JSON file.")
	}
	if colorIntrinsics.Height != 720 {
		t.Error("Color sensor height does not have the right value.")
	}
	if colorIntrinsics.Width != 1280 {
		t.Error("Color sensor width does not have the right value.")
	}

	if colorIntrinsics.Fx != 900.538 {
		t.Error("Color sensor focal distance in x does not have the right value.")
	}
	if colorIntrinsics.Fy != 900.818 {
		t.Error("Color sensor focal distance in y does not have the right value.")
	}
	// check sensorParams sensor parameters values
	sensorParams, err3 := NewDepthColorIntrinsicsExtrinsicsFromJSONFile(jsonFilePath)
	if err3 != nil {
		t.Fatal("Could not read parameters from JSON file.")
	}
	gtRotation := []float64{0.999958, -0.00838489, 0.00378392, 0.00824708, 0.999351, 0.0350734, -0.00407554, -0.0350407, 0.999378}

	if len(sensorParams.ExtrinsicD2C.RotationMatrix) != 9 {
		t.Error("Rotation Matrix should have 9 elements.")
	}
	for k := 0; k < len(sensorParams.ExtrinsicD2C.RotationMatrix); k++ {
		if sensorParams.ExtrinsicD2C.RotationMatrix[k] != gtRotation[k] {
			t.Error("Rotation matrix does not correspond to the GT one.")
		}
	}
	if len(sensorParams.ExtrinsicD2C.TranslationVector) != 3 {
		t.Error("Translation Vector should have 3 elements.")
	}

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
	if x2 != 0. {
		t.Error("x value for I rotation and {0,0,1} translation is not 0.")
	}
	if y2 != 0. {
		t.Error("y value for I rotation and {0,0,1} translation is not 0.")
	}
	if z2 != 2. {
		t.Error("z value for I rotation and {0,0,1} translation is not 2.")
	}

	t2 := []float64{0, 2, 0}
	extrinsics2 := Extrinsics{
		RotationMatrix:    rot1,
		TranslationVector: t2,
	}
	x3, y3, z3 := extrinsics2.TransformPointToPoint(x1, y1, z1)
	if x3 != 0. {
		t.Error("x value for I rotation and {0,2,0} translation is not 0.")
	}
	if y3 != 2. {
		t.Error("y value for I rotation and {0,2,0} translation is not 2.")
	}
	if z3 != 1. {
		t.Error("z value for I rotation and {0,2,0} translation is not 1.")
	}
	// Rotation in the (z,x) plane of 90 degrees
	rot2 := []float64{0, 0, 1, 0, 1, 0, 0, 0, -1}
	extrinsics3 := Extrinsics{
		RotationMatrix:    rot2,
		TranslationVector: t2,
	}
	x4, y4, z4 := extrinsics3.TransformPointToPoint(x1, y1, z1)
	if x4 != 1. {
		t.Error("x value for rotation z->x and {0,2,0} translation is not 1.")
	}
	if y4 != 2. {
		t.Error("y value for rotation z->x and {0,2,0} translation is not 2.")
	}
	if z4 != -1. {
		t.Error("z value for rotation z->x and {0,2,0} translation is not 0.")
	}
}
