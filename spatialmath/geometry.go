package spatialmath

import (
	"encoding/json"
	"fmt"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
)

// GeometryCreator provides a common way to instantiate Geometries.
type GeometryCreator interface {
	NewGeometry(Pose) Geometry
	Offset() Pose
	ToProtobuf() *commonpb.Geometry
	String() string
	json.Marshaler
}

// Geometry is an entry point with which to access all types of collision geometries.
type Geometry interface {
	Pose() Pose
	Vertices() []r3.Vector
	AlmostEqual(Geometry) bool
	Transform(Pose) Geometry
	ToProtobuf() *commonpb.Geometry
	CollidesWith(Geometry) (bool, error)
	DistanceFrom(Geometry) (float64, error)
	EncompassedBy(Geometry) (bool, error)
	Label() string
}

// GeometryType defines what geometry creator representations are known.
type GeometryType string

// The set of allowed representations for orientation.
const (
	UnknownType     = GeometryType("")
	BoxType         = GeometryType("box")
	SphereType      = GeometryType("sphere")
	PointType       = GeometryType("point")
	CollisionBuffer = 1e-8 // objects must be separated by this many mm to not be in collision
)

// GeometryConfig specifies the format of geometries specified through the configuration file.
type GeometryConfig struct {
	Type GeometryType `json:"type"`

	// parameters used for defining a box's rectangular cross-section
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`

	// parameter used for defining a sphere's radius'
	R float64 `json:"r"`

	// define an offset to position the geometry
	TranslationOffset r3.Vector         `json:"translation,omitempty"`
	OrientationOffset OrientationConfig `json:"orientation,omitempty"`

	Label string
}

// NewGeometryConfig creates a config for a Geometry from an offset Pose.
func NewGeometryConfig(gc GeometryCreator) (*GeometryConfig, error) {
	config := &GeometryConfig{}
	switch gcType := gc.(type) {
	case *boxCreator:
		config.Type = BoxType
		config.X = gc.(*boxCreator).halfSize.X * 2
		config.Y = gc.(*boxCreator).halfSize.Y * 2
		config.Z = gc.(*boxCreator).halfSize.Z * 2
		config.Label = gc.(*boxCreator).label
	case *sphereCreator:
		config.Type = SphereType
		config.R = gc.(*sphereCreator).radius
		config.Label = gc.(*sphereCreator).label
	case *pointCreator:
		config.Type = PointType
		config.Label = gc.(*pointCreator).label
	default:
		return nil, fmt.Errorf("%w %s", ErrGeometryTypeUnsupported, fmt.Sprintf("%T", gcType))
	}
	offset := gc.Offset()
	o := offset.Orientation()
	config.TranslationOffset = Compose(NewPoseFromOrientation(r3.Vector{}, OrientationInverse(o)), offset).Point()
	orientationConfig, err := NewOrientationConfig(o)
	if err != nil {
		return nil, err
	}
	config.OrientationOffset = *orientationConfig
	return config, nil
}

// ParseConfig converts a GeometryConfig into the correct GeometryCreator type, as specified in its Type field.
func (config *GeometryConfig) ParseConfig() (GeometryCreator, error) {
	// determine offset to use
	orientation, err := config.OrientationOffset.ParseConfig()
	if err != nil {
		return nil, err
	}
	offset := Compose(NewPoseFromOrientation(r3.Vector{}, orientation), NewPoseFromPoint(config.TranslationOffset))

	// build GeometryCreator depending on specified type
	switch config.Type {
	case BoxType:
		return NewBoxCreator(r3.Vector{X: config.X, Y: config.Y, Z: config.Z}, offset, config.Label)
	case SphereType:
		return NewSphereCreator(config.R, offset, config.Label)
	case PointType:
		return NewPointCreator(offset, config.Label), nil
	case UnknownType:
		// no type specified, iterate through supported types and try to infer intent
		if creator, err := NewBoxCreator(r3.Vector{X: config.X, Y: config.Y, Z: config.Z}, offset, config.Label); err == nil {
			return creator, nil
		}
		if creator, err := NewSphereCreator(config.R, offset, config.Label); err == nil {
			return creator, nil
		}
		// never try to infer point geometry if nothing is specified
	}
	return nil, fmt.Errorf("%w %s", ErrGeometryTypeUnsupported, string(config.Type))
}

// NewGeometryFromProto instantiates a new Geometry from a protobuf Geometry message.
func NewGeometryFromProto(geometry *commonpb.Geometry) (Geometry, error) {
	pose := NewPoseFromProtobuf(geometry.Center)
	if box := geometry.GetBox().GetDimsMm(); box != nil {
		return NewBox(pose, r3.Vector{X: box.X, Y: box.Y, Z: box.Z}, geometry.Label)
	}
	if sphere := geometry.GetSphere(); sphere != nil {
		if sphere.RadiusMm == 0 {
			return NewPoint(pose.Point(), geometry.Label), nil
		}
		return NewSphere(pose.Point(), sphere.RadiusMm, geometry.Label)
	}
	return nil, ErrGeometryTypeUnsupported
}

// NewGeometryCreatorFromProto instantiates a new GeometryCreator from a protobuf Geometry message.
func NewGeometryCreatorFromProto(geometry *commonpb.Geometry) (GeometryCreator, error) {
	pose := NewPoseFromProtobuf(geometry.Center)
	if box := geometry.GetBox().GetDimsMm(); box != nil {
		return NewBoxCreator(r3.Vector{X: box.X, Y: box.Y, Z: box.Z}, pose, geometry.Label)
	}
	if sphere := geometry.GetSphere(); sphere != nil {
		if sphere.RadiusMm == 0 {
			return NewPointCreator(pose, geometry.Label), nil
		}
		return NewSphereCreator(sphere.RadiusMm, pose, geometry.Label)
	}
	return nil, ErrGeometryTypeUnsupported
}

// ToProtobuf converts a GeometryConfig to Protobuf.
func (config *GeometryConfig) ToProtobuf() (*commonpb.Geometry, error) {
	creator, err := config.ParseConfig()
	if err != nil {
		return nil, err
	}
	return creator.ToProtobuf(), nil
}
