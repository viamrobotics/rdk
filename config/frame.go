package config

// Translation is the translation between two objects in the grid system
// There are always in millimeters
type Translation struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Orientation is the orientation between two objects
// This is represented as an orientation vector,
// With theta being in degrees and being the rotation around the orientation vector
type Orientation struct {
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	Z  float64 `json:"z"`
	TH float64 `json:"th"`
}
