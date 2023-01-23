package spatialmath

import (
	"encoding/json"
	"fmt"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
)

// Geometry is an entry point with which to access all types of collision geometries.
type Geometry interface {
	Pose() Pose
	AlmostEqual(Geometry) bool
	Transform(Pose) Geometry
	ToProtobuf() *commonpb.Geometry
	CollidesWith(Geometry) (bool, error)
	// If DistanceFrom is negative, it represents the penetration depth of the two geometries, which are in collision.
	// Penetration depth magnitude is defined as the minimum translation which would result in the geometries not colliding.
	// For certain entity pairs (box-box) this may be a conservative estimate of separation distance rather than exact.
	DistanceFrom(Geometry) (float64, error)
	EncompassedBy(Geometry) (bool, error)
	Label() string  // Label is the name of the geometry
	String() string // String is a string representation of the geometry data structure
	ToPoints(float64) []r3.Vector
	json.Marshaler
}

// GeometryType defines what geometry creator representations are known.
type GeometryType string

// The set of allowed representations for orientation.
const (
	UnknownType     = GeometryType("")
	BoxType         = GeometryType("box")
	SphereType      = GeometryType("sphere")
	CapsuleType     = GeometryType("capsule")
	PointType       = GeometryType("point")
	CollisionBuffer = 1e-8 // objects must be separated by this many mm to not be in collision

	// Point density corresponding to how many points per square mm.
	defaultPointDensity = .5
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

	// parameter used for defining a capsule's length
	L float64 `json:"l"`

	// define an offset to position the geometry
	TranslationOffset r3.Vector         `json:"translation,omitempty"`
	OrientationOffset OrientationConfig `json:"orientation,omitempty"`

	Label string
}

// NewGeometryConfig creates a config for a Geometry from an offset Pose.
func NewGeometryConfig(gc Geometry) (*GeometryConfig, error) {
	config := &GeometryConfig{}
	switch gcType := gc.(type) {
	case *box:
		config.Type = BoxType
		config.X = gc.(*box).halfSize[0] * 2
		config.Y = gc.(*box).halfSize[1] * 2
		config.Z = gc.(*box).halfSize[2] * 2
		config.Label = gc.(*box).label
	case *sphere:
		config.Type = SphereType
		config.R = gc.(*sphere).radius
		config.Label = gc.(*sphere).label
	case *capsule:
		config.Type = SphereType
		config.R = gc.(*capsule).radius
		config.L = gc.(*capsule).length
		config.Label = gc.(*capsule).label
	case *point:
		config.Type = PointType
		config.Label = gc.(*point).label
	default:
		return nil, fmt.Errorf("%w %s", ErrGeometryTypeUnsupported, fmt.Sprintf("%T", gcType))
	}
	offset := gc.Pose()
	o := offset.Orientation()
	config.TranslationOffset = Compose(NewPoseFromOrientation(OrientationInverse(o)), offset).Point()
	orientationConfig, err := NewOrientationConfig(o)
	if err != nil {
		return nil, err
	}
	config.OrientationOffset = *orientationConfig
	return config, nil
}

// ParseConfig converts a GeometryConfig into the correct GeometryCreator type, as specified in its Type field.
func (config *GeometryConfig) ParseConfig() (Geometry, error) {
	// determine offset to use
	orientation, err := config.OrientationOffset.ParseConfig()
	if err != nil {
		return nil, err
	}
	offset := Compose(NewPoseFromOrientation(orientation), NewPoseFromPoint(config.TranslationOffset))

	// build GeometryCreator depending on specified type
	switch config.Type {
	case BoxType:
		return NewBox(offset, r3.Vector{X: config.X, Y: config.Y, Z: config.Z}, config.Label)
	case SphereType:
		return NewSphere(offset, config.R, config.Label)
	case CapsuleType:
		return NewCapsule(offset, config.R, config.L, config.Label)
	case PointType:
		return NewPoint(offset.Point(), config.Label), nil
	case UnknownType:
		// no type specified, iterate through supported types and try to infer intent
		boxDims := r3.Vector{X: config.X, Y: config.Y, Z: config.Z}
		if boxDims.Norm() > 0 {
			if creator, err := NewBox(offset, boxDims, config.Label); err == nil {
				return creator, nil
			}
		} else if config.L != 0 {
			if creator, err := NewCapsule(offset, config.R, config.L, config.Label); err == nil {
				return creator, nil
			}
		} else if creator, err := NewSphere(offset, config.R, config.Label); err == nil {
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
	if capsule := geometry.GetCapsule(); capsule != nil {
		return NewCapsule(pose, capsule.RadiusMm, capsule.LengthMm, geometry.Label)
	}
	if sphere := geometry.GetSphere(); sphere != nil {
		if sphere.RadiusMm == 0 {
			return NewPoint(pose.Point(), geometry.Label), nil
		}
		return NewSphere(pose, sphere.RadiusMm, geometry.Label)
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
