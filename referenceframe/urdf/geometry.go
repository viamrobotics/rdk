package urdf

import (
	"encoding/xml"
	"fmt"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var errGeometryTypeUnsupported = errors.New("unsupported Geometry type")

// collision is a struct which details the XML used in a URDF collision geometry
type collision struct {
	XMLName  xml.Name `xml:"collision"`
	Origin   *pose    `xml:"origin"`
	Geometry struct {
		XMLName xml.Name `xml:"geometry"`
		Box     *box     `xml:"box,omitempty"`
		Sphere  *sphere  `xml:"sphere,omitempty"`
	} `xml:"geometry"`
}

func newCollision(g spatialmath.Geometry) (*collision, error) {
	urdf := &collision{
		Origin: newPose(g.Pose()),
	}
	switch gType := g.(type) {
	case *spatialmath.box:
		urdf.Geometry.Box = newURDFBox(gType)
	case *spatialmath.sphere:
		urdf.Geometry.Sphere = newURDFSphere(gType)
	default:
		return nil, fmt.Errorf("%w %s", errGeometryTypeUnsupported, fmt.Sprintf("%T", gType))
	}
	return urdf, nil
}

func (c *collision) parse() (spatialmath.Geometry, error) {
	switch {
	case c.Geometry.Box != nil:
		dims := spaceDelimitedStringToFloatSlice(c.Geometry.Box.Size)
		return spatialmath.NewBox(
			c.Origin.Parse(),
			r3.Vector{X: utils.MetersToMM(dims[0]), Y: utils.MetersToMM(dims[1]), Z: utils.MetersToMM(dims[2])},
			"",
		)
	case c.Geometry.Sphere != nil:
		return spatialmath.NewSphere(c.Origin.Parse(), utils.MetersToMM(c.Geometry.Sphere.Radius), "")
	default:
		return nil, errors.Errorf("couldn't parse xml: no geometry defined")
	}
}

type box struct {
	XMLName xml.Name `xml:"box"`
	Size    string   `xml:"size,attr"` // "x y z" format, in meters
}

func newURDFBox(b *spatialmath.box) *box {
	return &box{Size: fmt.Sprintf("%f %f %f",
		utils.MMToMeters(2*b.halfSize[0]),
		utils.MMToMeters(2*b.halfSize[1]),
		utils.MMToMeters(2*b.halfSize[2]),
	)}
}

type sphere struct {
	XMLName xml.Name `xml:"sphere"`
	Radius  float64  `xml:"radius,attr"` // in meters
}

func newURDFSphere(s *spatialmath.sphere) *sphere {
	return &sphere{Radius: utils.MMToMeters(s.radius)}
}
