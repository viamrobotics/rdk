package config

import (
	spatial "go.viam.com/core/spatialmath"
)

// Translation is the translation between two objects in the grid system. It is always in millimeters.
type Translation struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// FrameConfig the pose and parent of the frame that will be created.
type FrameConfig struct {
	Parent      string                         `json:"parent"`
	Translation Translation                    `json:"translation"`
	OVDegrees   *spatial.OrientationVecDegrees `json:"ovdegrees"`
	OVRadians   *spatial.OrientationVec        `json:"ovradians"`
	AxisAngles  *spatial.R4AA                  `json:"axisangles"`
	EulerAngles *spatial.EulerAngles           `json:"eulerangles"`
}

// Orientation returns the orientation of the object from the config, or an empty orientation if no orientation specified
// or if orientation not recognized.
func (fc *FrameConfig) Orientation() spatial.Orientation {
	if fc.OVDegrees != nil {
		return spatial.NewOrientationFromOVD(fc.OVDegrees)
	} else if fc.OVRadians != nil {
		return spatial.NewOrientationFromOV(fc.OVRadians)
	} else if fc.AxisAngles != nil {
		return spatial.NewOrientationFromAxisAngles(fc.AxisAngles)
	} else if fc.EulerAngles != nil {
		return spatial.NewOrientationFromEulerAngles(fc.EulerAngles)
	}
	return spatial.NewZeroOrientation()
}
