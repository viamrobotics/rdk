package referenceframe

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

const (
	// urdfPackagePrefix is the URI scheme used in URDF files to reference ROS packages
	urdfPackagePrefix = "package://"
)

var errGeometryTypeUnsupported = errors.New("unsupported Geometry type")

// normalizeURDFMeshPath converts a URDF mesh path (which may use package:// URI) to a relative path.
// For example: "package://ur_description/meshes/base.stl" -> "meshes/base.stl"
func normalizeURDFMeshPath(meshPath string) string {
	// Handle package:// URIs
	// Strip the package prefix to get the relative path
	if strings.HasPrefix(meshPath, urdfPackagePrefix) {
		meshPath = strings.TrimPrefix(meshPath, urdfPackagePrefix)
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
		XMLName  xml.Name  `xml:"geometry"`
		Box      *box      `xml:"box,omitempty"`
		Sphere   *sphere   `xml:"sphere,omitempty"`
		Cylinder *cylinder `xml:"cylinder,omitempty"`
		Mesh     *mesh     `xml:"mesh,omitempty"`
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

type cylinder struct {
	XMLName xml.Name `xml:"cylinder"`
	Radius  float64  `xml:"radius,attr"` // in meters
	Length  float64  `xml:"length,attr"` // in meters
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
	case spatialmath.CapsuleType:
		return nil, errors.New("use newCollisions for capsule geometries")
	default:
		return nil, fmt.Errorf("%w %s", errGeometryTypeUnsupported, fmt.Sprintf("%T", cfg.Type))
	}
	return urdf, nil
}

// newCollisions converts a geometry to URDF collision elements.
// For capsules, this returns 3 collisions (cylinder + 2 spheres).
// For other types, it returns a single collision.
func newCollisions(g spatialmath.Geometry) ([]collision, error) {
	cfg, err := spatialmath.NewGeometryConfig(g)
	if err != nil {
		return nil, err
	}

	if cfg.Type == spatialmath.CapsuleType {
		colls, err := newCollisionsFromCapsule(g)
		if err != nil {
			return nil, err
		}
		result := make([]collision, 0, len(colls))
		for _, c := range colls {
			result = append(result, *c)
		}
		return result, nil
	}

	coll, err := newCollision(g)
	if err != nil {
		return nil, err
	}
	return []collision{*coll}, nil
}

// tryParseCapsuleFromCollisions checks if a slice of collision elements represents a capsule
// (one cylinder + two spheres positioned at the cylinder's ends with matching radii).
// Returns the capsule geometry if the pattern matches, nil otherwise.
func tryParseCapsuleFromCollisions(collisions []collision) (spatialmath.Geometry, error) {
	if len(collisions) != 3 {
		return nil, nil
	}

	// Find the cylinder and spheres
	var cyl *collision
	var spheres []*collision
	for i := range collisions {
		c := &collisions[i]
		switch {
		case c.Geometry.Cylinder != nil:
			if cyl != nil {
				return nil, nil // more than one cylinder
			}
			cyl = c
		case c.Geometry.Sphere != nil:
			spheres = append(spheres, c)
		default:
			return nil, nil // contains non-cylinder/sphere geometry
		}
	}

	if cyl == nil || len(spheres) != 2 {
		return nil, nil
	}

	// Get dimensions (convert from meters to mm)
	cylRadius := utils.MetersToMM(cyl.Geometry.Cylinder.Radius)
	cylLength := utils.MetersToMM(cyl.Geometry.Cylinder.Length)
	sphere1Radius := utils.MetersToMM(spheres[0].Geometry.Sphere.Radius)
	sphere2Radius := utils.MetersToMM(spheres[1].Geometry.Sphere.Radius)

	// Check that all radii match
	const tolerance = 1e-6
	if math.Abs(cylRadius-sphere1Radius) > tolerance || math.Abs(cylRadius-sphere2Radius) > tolerance {
		return nil, nil
	}

	// Get origins
	cylOrigin := spatialmath.NewZeroPose()
	if cyl.Origin != nil {
		cylOrigin = cyl.Origin.Parse()
	}
	sphere1Origin := spatialmath.NewZeroPose()
	if spheres[0].Origin != nil {
		sphere1Origin = spheres[0].Origin.Parse()
	}
	sphere2Origin := spatialmath.NewZeroPose()
	if spheres[1].Origin != nil {
		sphere2Origin = spheres[1].Origin.Parse()
	}

	// Check sphere positions: they should be at ±(cylLength/2) along the cylinder's Z-axis
	// relative to the cylinder's origin
	expectedOffset := cylLength / 2
	cylPt := cylOrigin.Point()
	s1Pt := sphere1Origin.Point()
	s2Pt := sphere2Origin.Point()

	// Calculate offsets from cylinder center
	s1Offset := s1Pt.Sub(cylPt)
	s2Offset := s2Pt.Sub(cylPt)

	// For a valid capsule, the spheres should be on opposite ends along the Z-axis
	// One should be at +expectedOffset and one at -expectedOffset (in Z)
	// and both should have ~0 offset in X and Y
	if math.Abs(s1Offset.X) > tolerance || math.Abs(s1Offset.Y) > tolerance ||
		math.Abs(s2Offset.X) > tolerance || math.Abs(s2Offset.Y) > tolerance {
		return nil, nil
	}

	// Check Z offsets match expected positions (one positive, one negative)
	if !((math.Abs(s1Offset.Z-expectedOffset) < tolerance && math.Abs(s2Offset.Z+expectedOffset) < tolerance) ||
		(math.Abs(s1Offset.Z+expectedOffset) < tolerance && math.Abs(s2Offset.Z-expectedOffset) < tolerance)) {
		return nil, nil
	}

	// Pattern matches! Create the capsule
	// Capsule length = cylinder length + 2*radius (total tip-to-tip)
	capsuleLength := cylLength + 2*cylRadius
	return spatialmath.NewCapsule(cylOrigin, cylRadius, capsuleLength, "")
}

// newCollisionsFromCapsule decomposes a capsule geometry into URDF collision elements
// (one cylinder + two spheres).
func newCollisionsFromCapsule(g spatialmath.Geometry) ([]*collision, error) {
	cfg, err := spatialmath.NewGeometryConfig(g)
	if err != nil {
		return nil, err
	}
	if cfg.Type != spatialmath.CapsuleType {
		return nil, errors.New("geometry is not a capsule")
	}

	radius := cfg.R
	length := cfg.L
	pose := g.Pose()

	// Cylinder length = capsule length - 2*radius
	cylLength := length - 2*radius

	// Sphere positions: at ±(cylLength/2) along Z-axis from capsule center
	sphereOffset := cylLength / 2

	// Create cylinder collision at capsule origin
	cylCollision := &collision{
		Origin: newPose(pose),
	}
	cylCollision.Geometry.Cylinder = &cylinder{
		Radius: utils.MMToMeters(radius),
		Length: utils.MMToMeters(cylLength),
	}

	// Create sphere at +Z
	sphere1Pose := spatialmath.Compose(pose, spatialmath.NewPoseFromPoint(r3.Vector{Z: sphereOffset}))
	sphere1Collision := &collision{
		Origin: newPose(sphere1Pose),
	}
	sphere1Collision.Geometry.Sphere = &sphere{
		Radius: utils.MMToMeters(radius),
	}

	// Create sphere at -Z
	sphere2Pose := spatialmath.Compose(pose, spatialmath.NewPoseFromPoint(r3.Vector{Z: -sphereOffset}))
	sphere2Collision := &collision{
		Origin: newPose(sphere2Pose),
	}
	sphere2Collision.Geometry.Sphere = &sphere{
		Radius: utils.MMToMeters(radius),
	}

	return []*collision{cylCollision, sphere1Collision, sphere2Collision}, nil
}

func (c *collision) toGeometry(meshMap map[string]*commonpb.Mesh) (spatialmath.Geometry, error) {
	// Get origin, defaulting to zero pose if not specified (optional in URDF)
	origin := spatialmath.NewZeroPose()
	if c.Origin != nil {
		origin = c.Origin.Parse()
	}

	switch {
	case c.Geometry.Box != nil:
		dims := spaceDelimitedStringToFloatSlice(c.Geometry.Box.Size)
		return spatialmath.NewBox(
			origin,
			r3.Vector{X: utils.MetersToMM(dims[0]), Y: utils.MetersToMM(dims[1]), Z: utils.MetersToMM(dims[2])},
			"",
		)
	case c.Geometry.Sphere != nil:
		return spatialmath.NewSphere(origin, utils.MetersToMM(c.Geometry.Sphere.Radius), "")
	case c.Geometry.Cylinder != nil:
		// Standalone cylinders are not natively supported in spatialmath.
		// Use the cylinder+two-spheres pattern to represent a capsule instead.
		return nil, errors.New("standalone cylinder geometry not supported; use cylinder + two spheres pattern for capsule")
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

		mesh, err := spatialmath.NewMeshFromProto(origin, protoMesh, "")
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
