package config

// A FrameType defines how the frame should be constructed
type FrameType string

const (
	FrameTypeStatic    = FrameType("static")
	FrameTypePrismatic = FrameType("prismatic")
	FrameTypeRevolute  = FrameType("revolute")
	FrameTypeModel     = FrameType("model")
)

// FrameConfig specifies what type of Frame should be created for the component
// Translation is the translation between two objects in the grid system. It is  always in millimeters
// Orientation is the orientation between two objects. This is represented as an orientation vector,
// With theta being in degrees and being the rotation around the orientation vector
type FrameConfig struct {
	Type        FrameType `json:"type"`
	Parent      string    `json:"parent"`
	Translation struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
		Z float64 `json:"z"`
	} `json:"translation"`
	SetOrientation bool     `json:"setorientation"`
	Orientation    struct { // for poses
		X float64 `json:"x"`
		Y float64 `json:"y"`
		Z float64 `json:"z"`
		T float64 `json:"th"`
	} `json:"orientation"`
	Axis struct { // for revolute frames
		X float64 `json:"x"`
		Y float64 `json:"y"`
		Z float64 `json:"z"`
	} `json:"axis"`
	Axes struct { // for prismatic frames
		X bool `json:"x"`
		Y bool `json:"y"`
		Z bool `json:"z"`
	} `json:"axes"`
	Min []float64 `json:"min"`
	Max []float64 `json:"max"`
}
