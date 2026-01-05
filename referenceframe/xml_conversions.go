package referenceframe

import (
	"encoding/xml"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var errGeometryTypeUnsupported = errors.New("unsupported Geometry type")

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
	Filename string   `xml:"filename,attr"` // path to mesh file (STL or PLY)
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
		// For mesh, we use the label as the filename if it looks like a path
		// Otherwise, we can't serialize back to URDF without more context
		if g.Label() != "" {
			urdf.Geometry.Mesh = &mesh{Filename: g.Label()}
		} else {
			return nil, errors.New("mesh geometry must have a label (filename) to be serialized to URDF")
		}
	default:
		return nil, fmt.Errorf("%w %s", errGeometryTypeUnsupported, fmt.Sprintf("%T", cfg.Type))
	}
	return urdf, nil
}

func (c *collision) toGeometry() (spatialmath.Geometry, error) {
	return c.toGeometryWithBasePath("")
}

func (c *collision) toGeometryWithBasePath(basePath string) (spatialmath.Geometry, error) {
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
		// Resolve mesh file path relative to URDF base directory
		meshPath := c.Geometry.Mesh.Filename

		// Handle package:// URIs from URDF files
		if strings.HasPrefix(meshPath, "package://") {
			// Strip "package://<package_name>/" and use the remaining path
			meshPath = strings.TrimPrefix(meshPath, "package://")
			// Find the first "/" to skip the package name
			if idx := strings.Index(meshPath, "/"); idx != -1 {
				meshPath = meshPath[idx+1:]
			}
			// Join with basePath
			if basePath != "" {
				meshPath = filepath.Join(basePath, meshPath)
			}
		} else if basePath != "" && !filepath.IsAbs(meshPath) {
			meshPath = filepath.Join(basePath, meshPath)
		}

		// Determine file type and load accordingly
		ext := strings.ToLower(filepath.Ext(meshPath))
		var mesh *spatialmath.Mesh
		var err error

		switch ext {
		case ".stl":
			mesh, err = spatialmath.NewMeshFromSTLFile(meshPath)
		case ".ply":
			mesh, err = spatialmath.NewMeshFromPLYFile(meshPath)
		default:
			return nil, errors.Errorf("unsupported mesh file format: %s (must be .stl or .ply)", ext)
		}

		if err != nil {
			return nil, errors.Wrapf(err, "failed to load mesh from %s", meshPath)
		}

		// Set label to the absolute mesh path so it can be reloaded later
		mesh.SetLabel(meshPath)
		// Apply the collision origin transform
		return mesh.Transform(c.Origin.Parse()).(*spatialmath.Mesh), nil
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
