package calibration

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
