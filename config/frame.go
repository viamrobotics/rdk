package config

// A FrameType defines how the frame should be constructed
type FrameType string

const (
	FrameTypeStatic = FrameType("static")
	FrameTypeModel  = FrameType("model")
)

// Translation is the translation between two objects in the grid system. It is  always in millimeters
type Translation struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Orientation is the orientation between two objects. This is represented as an orientation vector,
// With theta being in degrees and being the rotation around the orientation vector
type Orientation struct { // for poses
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	Z  float64 `json:"z"`
	TH float64 `json:"th"`
}

// FrameConfig specifies what type of Frame should be created for the component, and how.
type FrameConfig struct {
	Type           FrameType   `json:"type"`
	Parent         string      `json:"parent"`
	Translate      Translation `json:"translation"`
	SetOrientation bool        `json:"setorientation"`
	Orient         Orientation `json:"orientation"`
}
