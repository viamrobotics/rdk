package spatialmath

import (
	"encoding/json"
	"fmt"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
)

const (
	// objects must be separated by this many mm to not be in collision.
	defaultCollisionBufferMM = 1e-8

	// Point density corresponding to how many points per square mm.
	defaultPointDensity = .5
)

// Geometry is an interface defining a 3D solid.
type Geometry interface {
	// Pose returns the Pose of the center of the Geometry
	Pose() Pose

	// Transform returns a copy of the Geometry that has been transformed by the given Pose
	Transform(Pose) Geometry

	// CollidesWith returns a bool describing if the two geometries are within the given float of colliding with each other.
	// The 2nd return value is -1 if there is a collision, and if not, a lower bound of the distance between the objects.
	// It is meant to be computed quickly.
	// If it cannot be, should just return buffer
	CollidesWith(other Geometry, buffer float64) (bool, float64, error)

	// If DistanceFrom is negative, it represents the penetration depth of the two geometries, which are in collision.
	// Penetration depth magnitude is defined as the minimum translation which would result in the geometries not colliding.
	// For certain entity pairs (box-box) this may be a conservative estimate of separation distance rather than exact.
	DistanceFrom(Geometry) (float64, error)

	// EncompassedBy returns a bool describing if a given Geometry is completely encompassed by the Geometry passed as an argument.
	EncompassedBy(Geometry) (bool, error)

	// SetLabel sets the name of the geometry
	SetLabel(string)

	// Label returns the name of the geometry
	Label() string

	// ToPoints returns a vector of points that together represent a point cloud of the Geometry
	ToPoints(float64) []r3.Vector

	// ToProtobuf converts a Geometry to its protobuf representation.
	ToProtobuf() *commonpb.Geometry

	Hash() int

	json.Marshaler
}

// GeometryType defines what geometry representations are known.
type GeometryType string

// The set of allowed representations for the Type in a geometry config.
const (
	UnknownType = GeometryType("")
	BoxType     = GeometryType("box")
	SphereType  = GeometryType("sphere")
	CapsuleType = GeometryType("capsule")
	PointType   = GeometryType("point")
	MeshType    = GeometryType("mesh")
)

// GeometryConfig specifies the format of geometries specified through JSON configuration files.
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

	// parameters used for defining a mesh
	MeshData        []byte `json:"mesh_data,omitempty"`         // Binary mesh file data
	MeshContentType string `json:"mesh_content_type,omitempty"` // e.g., "stl", "ply"
	MeshFilePath    string `json:"mesh_file_path,omitempty"`    // Original URDF mesh path (e.g., "meshes/ur20/collision/base.stl")

	// define an offset to position the geometry
	TranslationOffset r3.Vector         `json:"translation,omitempty"`
	OrientationOffset OrientationConfig `json:"orientation,omitempty"`

	Label string
}

// NewGeometryConfig creates a config for a Geometry from an offset Pose.
func NewGeometryConfig(g Geometry) (*GeometryConfig, error) {
	config := &GeometryConfig{}
	switch gType := g.(type) {
	case *box:
		config.Type = BoxType
		config.X = gType.halfSize[0] * 2
		config.Y = gType.halfSize[1] * 2
		config.Z = gType.halfSize[2] * 2
		config.Label = gType.label
	case *sphere:
		config.Type = SphereType
		config.R = gType.radius
		config.Label = gType.label
	case *capsule:
		config.Type = CapsuleType
		config.R = gType.radius
		config.L = gType.length
		config.Label = gType.label
	case *point:
		config.Type = PointType
		config.Label = gType.label
	case *Mesh:
		config.Type = MeshType
		config.MeshData = gType.rawBytes
		config.MeshContentType = string(gType.fileType)
		config.MeshFilePath = gType.originalFilePath
		config.Label = gType.label
	default:
		return nil, fmt.Errorf("%w %s", errGeometryTypeUnsupported, fmt.Sprintf("%T", gType))
	}
	offset := g.Pose()
	o := offset.Orientation()
	config.TranslationOffset = offset.Point()
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
	offset := NewPose(config.TranslationOffset, orientation)

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
	case MeshType:
		if len(config.MeshData) == 0 {
			return nil, fmt.Errorf("mesh geometry requires mesh data")
		}
		// Create proto Mesh and use NewMeshFromProto
		protoMesh := &commonpb.Mesh{
			Mesh:        config.MeshData,
			ContentType: config.MeshContentType,
		}
		mesh, err := NewMeshFromProto(offset, protoMesh, config.Label)
		if err != nil {
			return nil, err
		}
		// Preserve the original file path for round-tripping
		if config.MeshFilePath != "" {
			mesh.SetOriginalFilePath(config.MeshFilePath)
		}
		return mesh, nil
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
	return nil, fmt.Errorf("%w: %s", errGeometryTypeUnsupported, string(config.Type))
}

// ToProtobuf converts a GeometryConfig to Protobuf.
func (config *GeometryConfig) ToProtobuf() (*commonpb.Geometry, error) {
	creator, err := config.ParseConfig()
	if err != nil {
		return nil, err
	}
	return creator.ToProtobuf(), nil
}

// GeometriesAlmostEqual returns a bool describing if the two input Geometries are equal.
func GeometriesAlmostEqual(a, b Geometry) bool {
	if a == nil {
		return b == nil
	} else if b == nil {
		return false
	}

	switch gType := a.(type) {
	case *box:
		return gType.almostEqual(b)
	case *sphere:
		return gType.almostEqual(b)
	case *capsule:
		return gType.almostEqual(b)
	case *point:
		return gType.almostEqual(b)
	default:
		return false
	}
}
