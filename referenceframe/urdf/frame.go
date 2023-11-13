package urdf

import (
	"encoding/xml"
	"fmt"

	"go.viam.com/rdk/spatialmath"
)

type frame struct {
	Link string `xml:"link,attr"`
}

type limit struct {
	XMLName xml.Name `xml:"limit"`
	Lower   float64  `xml:"lower,attr"` // translation limits are in meters, revolute limits are in radians
	Upper   float64  `xml:"upper,attr"` // translation limits are in meters, revolute limits are in radians
}

type axis struct {
	XMLName xml.Name `xml:"axis"`
	XYZ     string   `xml:"xyz,attr"` // "x y z" format, in meters
}

func newAxis(a spatialmath.AxisConfig) *axis {
	return &axis{XYZ: fmt.Sprintf("%f %f %f", a.X, a.Y, a.Z)}
}

func (a *axis) Parse() spatialmath.AxisConfig {
	jointAxes := spaceDelimitedStringToFloatSlice(a.XYZ)
	return spatialmath.AxisConfig{jointAxes[0], jointAxes[1], jointAxes[2]}
}
