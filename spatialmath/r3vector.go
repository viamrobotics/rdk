package spatialmath

import (
	"encoding/xml"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/utils"
)

// R3VectorAlmostEqual compares two r3.Vector objects and returns if the all elementwise differences are less than epsilon.
func R3VectorAlmostEqual(a, b r3.Vector, epsilon float64) bool {
	return math.Abs(a.X-b.X) < epsilon && math.Abs(a.Y-b.Y) < epsilon && math.Abs(a.Z-b.Z) < epsilon
}

// AxisConfig represents the configuration format representing an axis.
type AxisConfig r3.Vector

// NewAxisConfig constructs a config from an R4AA.
func NewAxisConfig(axis R4AA) *AxisConfig {
	return &AxisConfig{axis.RX, axis.RY, axis.RZ}
}

// ParseConfig converts an AxisConfig into an R4AA object.
func (a AxisConfig) ParseConfig() R4AA {
	return R4AA{RX: a.X, RY: a.Y, RZ: a.Z}
}

type URDFAxis struct {
	XMLName xml.Name `xml:"axis"`
	XYZ     string   `xml:"xyz,attr"` // "x y z" format, in meters
}

func NewURDFAxis(axis AxisConfig) *URDFAxis {
	return &URDFAxis{XYZ: fmt.Sprintf("%f %f %f", axis.X, axis.Y, axis.Z)}
}

func (urdf *URDFAxis) Parse() AxisConfig {
	jointAxes := utils.SpaceDelimitedStringToFloatSlice(urdf.XYZ)
	return AxisConfig{jointAxes[0], jointAxes[1], jointAxes[2]}
}
