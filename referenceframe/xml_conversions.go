package referenceframe

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"

	commonpb "go.viam.com/api/common/v1"
)

var errGeometryTypeUnsupported = errors.New("unsupported Geometry type")

// normalizeURDFMeshPath converts a URDF mesh path (which may use package:// URI) to a relative path.
// For example: "package://ur_description/meshes/base.stl" -> "meshes/base.stl"
func normalizeURDFMeshPath(meshPath string) string {
	// Handle package:// URIs
	// Strip the package prefix to get the relative path
	if strings.HasPrefix(meshPath, "package://") {
		meshPath = strings.TrimPrefix(meshPath, "package://")
		// Remove the package name part (e.g., "ur_description/meshes/..." -> "meshes/...")
		if idx := strings.Index(meshPath, "/"); idx != -1 {
			meshPath = meshPath[idx+1:]
		}
	}
	return meshPath
}

// collision is a struct which details the XML used in a URDF collision geometry.
type collision struct {
	XMLName  xml.Name `xml:"collision"`
	Origin   *pose    `xml:"origin"`
	Geometry struct {
		XMLName xml.Name `xml:"geometry"`
		Box     *box     `xml:"box,omitempty"`
		Sphere  *sphere  `xml:"sphere,omitempty"`
		Mesh    *mesh    `xml:"mesh,omitempty"`
	} `xml:"geometry"`
}

type box struct {
	XMLName xml.Name `xml:"box"`
	Size    string   `xml:"size,attr"` // "x y z" format, in meters
}

type sphere struct {
	XMLName xml.Name `xml:"sphere"`
	Radius  float64  `xml:"radius,attr"` // in meters
}

type mesh struct {
	XMLName  xml.Name `xml:"mesh"`
	Filename string   `xml:"filename,attr"`
}

func newCollision(g spatialmath.Geometry) (*collision, error) {
	cfg, err := spatialmath.NewGeometryConfig(g)
	if err != nil {
		return nil, err
	}
	urdf := &collision{
		Origin: newPose(g.Pose()),
	}
	//nolint:exhaustive
	switch cfg.Type {
	case spatialmath.BoxType:
		urdf.Geometry.Box = &box{Size: fmt.Sprintf("%f %f %f", utils.MMToMeters(cfg.X), utils.MMToMeters(cfg.Y), utils.MMToMeters(cfg.Z))}
	case spatialmath.SphereType:
		urdf.Geometry.Sphere = &sphere{Radius: utils.MMToMeters(cfg.R)}
	case spatialmath.MeshType:
		if cfg.MeshFilePath == "" {
			return nil, errors.New("mesh geometry does not have an original file path set")
		}
		urdf.Geometry.Mesh = &mesh{Filename: cfg.MeshFilePath}
	default:
		return nil, fmt.Errorf("%w %s", errGeometryTypeUnsupported, fmt.Sprintf("%T", cfg.Type))
	}
	return urdf, nil
}

func (c *collision) toGeometry(meshMap map[string]*commonpb.Mesh) (spatialmath.Geometry, error) {
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
	case c.Geometry.Mesh != nil:
		meshPath := normalizeURDFMeshPath(c.Geometry.Mesh.Filename)

		// Check if mesh map is provided
		if meshMap == nil {
			return nil, fmt.Errorf("mesh geometry requires mesh map to be provided, but got nil for mesh: %s", meshPath)
		}

		// Look up mesh proto in the provided map
		protoMesh, ok := meshMap[meshPath]
		if !ok {
			return nil, fmt.Errorf("mesh file not found in mesh map: %s", meshPath)
		}

		mesh, err := spatialmath.NewMeshFromProto(c.Origin.Parse(), protoMesh, "")
		if err != nil {
			return nil, err
		}
		// Store the original mesh path for round-tripping
		mesh.SetOriginalFilePath(meshPath)
		return mesh, nil
	default:
		return nil, errors.New("couldn't parse xml: no geometry defined")
	}
}

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

func (a *axis) Parse() spatialmath.AxisConfig {
	jointAxes := spaceDelimitedStringToFloatSlice(a.XYZ)
	return spatialmath.AxisConfig{X: jointAxes[0], Y: jointAxes[1], Z: jointAxes[2]}
}

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

// spaceDelimitedStringToFloatSlice is a helper method to split up space-delimited fields in a string and converts them to floats.
func spaceDelimitedStringToFloatSlice(s string) []float64 {
	var converted []float64
	slice := strings.Fields(s)
	for _, value := range slice {
		value, err := strconv.ParseFloat(value, 64)
		if err != nil {
			value = math.NaN()
		}
		converted = append(converted, value)
	}
	return converted
}
