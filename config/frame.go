package config

// A FrameType defines how the frame should be constructed
type FrameType string

const (
	FrameTypeStatic    = FrameType("static")
	FrameTypePrismatic = FrameType("prismatic")
	FrameTypeRevolute  = FrameType("revolute")
	FrameTypeModel     = FrameType("model")
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

// RevoluteAxis is the axis around which a revolute frame rotates around.
type RevoluteAxis struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// PrimasticAxes are a boolean set of the dimensions the gantry, etc can move in.
type PrismaticAxes struct {
	X bool `json:"x"`
	Y bool `json:"y"`
	Z bool `json:"z"`
}

// FrameConfig specifies what type of Frame should be created for the component, and how.
type FrameConfig struct {
	Type           FrameType     `json:"type"`
	Parent         string        `json:"parent"`
	Translate      Translation   `json:"translation"`
	SetOrientation bool          `json:"setorientation"`
	Orient         Orientation   `json:"orientation"`
	Axis           RevoluteAxis  `json:"axis"`
	Axes           PrismaticAxes `json:"axes"`
	Min            []float64     `json:"min"`
	Max            []float64     `json:"max"`
}
