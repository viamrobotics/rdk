package urdf

import (
	"encoding/xml"
	"fmt"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

type pose struct {
	XMLName xml.Name `xml:"origin"`
	RPY     string   `xml:"rpy,attr"` // Fixed frame angle "r p y" format, in radians
	XYZ     string   `xml:"xyz,attr"` // "x y z" format, in meters
}

func newPose(p spatialmath.Pose) *pose {
	pt := p.Point()
	o := p.Orientation().EulerAngles()
	return &pose{
		XYZ: fmt.Sprintf("%f %f %f", utils.MMToMeters(pt.X), utils.MMToMeters(pt.Y), utils.MMToMeters(pt.Z)),
		RPY: fmt.Sprintf("%f %f %f", o.Roll, o.Pitch, o.Yaw),
	}
}

func (p *pose) Parse() spatialmath.Pose {
	// Offset for the geometry origin from the reference link origin
	xyz := spaceDelimitedStringToFloatSlice(p.XYZ)
	rpy := spaceDelimitedStringToFloatSlice(p.RPY)
	return spatialmath.NewPose(
		r3.Vector{X: utils.MetersToMM(xyz[0]), Y: utils.MetersToMM(xyz[1]), Z: utils.MetersToMM(xyz[2])},
		&spatialmath.EulerAngles{Roll: rpy[0], Pitch: rpy[1], Yaw: rpy[2]},
	)
}
