// Package referenceframe defines the api and does the math of translating between reference frames
// Useful for if you have a camera, connected to a gripper, connected to an arm,
// and need to translate the camera reference frame to the arm reference frame,
// if you've found something in the camera, and want to move the gripper + arm to get it.
package referenceframe

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// Limit represents the limits of motion for a referenceframe.
type Limit struct {
	Min float64
	Max float64
}

func limitsAlmostEqual(a, b []Limit) bool {
	if len(a) != len(b) {
		return false
	}

	const epsilon = 1e-5
	for idx, x := range a {
		if !utils.Float64AlmostEqual(x.Min, b[idx].Min, epsilon) ||
			!utils.Float64AlmostEqual(x.Max, b[idx].Max, epsilon) {
			return false
		}
	}

	return true
}

// RestrictedRandomFrameInputs will produce a list of valid, in-bounds inputs for the frame, restricting the range to
// `lim` percent of the limits.
func RestrictedRandomFrameInputs(m Frame, rSeed *rand.Rand, lim float64) []Input {
	if rSeed == nil {
		//nolint:gosec
		rSeed = rand.New(rand.NewSource(1))
	}
	dof := m.DoF()
	pos := make([]Input, 0, len(dof))
	for _, limit := range dof {
		l, u := limit.Min, limit.Max

		// Default to [-999,999] as range if limits are infinite
		if l == math.Inf(-1) {
			l = -999
		}
		if u == math.Inf(1) {
			u = 999
		}

		jRange := math.Abs(u - l)
		pos = append(pos, Input{lim * (rSeed.Float64()*jRange + l)})
	}
	return pos
}

// RandomFrameInputs will produce a list of valid, in-bounds inputs for the referenceframe.
func RandomFrameInputs(m Frame, rSeed *rand.Rand) []Input {
	if rSeed == nil {
		//nolint:gosec
		rSeed = rand.New(rand.NewSource(1))
	}
	dof := m.DoF()
	pos := make([]Input, 0, len(dof))
	for _, lim := range dof {
		l, u := lim.Min, lim.Max

		// Default to [-999,999] as range if limits are infinite
		if l == math.Inf(-1) {
			l = -999
		}
		if u == math.Inf(1) {
			u = 999
		}

		jRange := math.Abs(u - l)
		pos = append(pos, Input{rSeed.Float64()*jRange + l})
	}
	return pos
}

// Frame represents a reference frame, e.g. an arm, a joint, a gripper, a board, etc.
type Frame interface {
	// Name returns the name of the referenceframe.
	Name() string

	// Transform is the pose (rotation and translation) that goes FROM current frame TO parent's referenceframe.
	Transform([]Input) (spatial.Pose, error)

	// Geometries returns a map between names and geometries for the reference frame and any intermediate frames that
	// may be defined for it, e.g. links in an arm. If a frame does not have a geometryCreator it will not be added into the map
	Geometries([]Input) (map[string]spatial.Geometry, error)

	// DoF will return a slice with length equal to the number of joints/degrees of freedom.
	// Each element describes the min and max movement limit of that joint/degree of freedom.
	// For robot parts that don't move, it returns an empty slice.
	DoF() []Limit

	// AlmostEquals returns if the otherFrame is close to the referenceframe.
	// differences should just be things like floating point inprecision
	AlmostEquals(otherFrame Frame) bool

	json.Marshaler
}

// a static Frame is a simple corrdinate system that encodes a fixed translation and rotation
// from the current Frame to the parent referenceframe.
type staticFrame struct {
	name            string
	transform       spatial.Pose
	geometryCreator spatial.GeometryCreator
}

// NewStaticFrame creates a frame given a pose relative to its parent. The pose is fixed for all time.
// Pose is not allowed to be nil.
func NewStaticFrame(name string, pose spatial.Pose) (Frame, error) {
	if pose == nil {
		return nil, errors.New("pose is not allowed to be nil")
	}
	return &staticFrame{name, pose, nil}, nil
}

// NewZeroStaticFrame creates a frame with no translation or orientation changes.
func NewZeroStaticFrame(name string) Frame {
	return &staticFrame{name, spatial.NewZeroPose(), nil}
}

// NewStaticFrameWithGeometry creates a frame given a pose relative to its parent.  The pose is fixed for all time.
// It also has an associated geometryCreator representing the space that it occupies in 3D space.  Pose is not allowed to be nil.
func NewStaticFrameWithGeometry(name string, pose spatial.Pose, geometryCreator spatial.GeometryCreator) (Frame, error) {
	if pose == nil {
		return nil, errors.New("pose is not allowed to be nil")
	}
	return &staticFrame{name, pose, geometryCreator}, nil
}

// NewStaticFrameFromFrame creates a frame given a pose relative to its parent.  The pose is fixed for all time.
// It inherits its name and geometryCreator properties from the specified Frame. Pose is not allowed to be nil.
func NewStaticFrameFromFrame(frame Frame, pose spatial.Pose) (Frame, error) {
	if pose == nil {
		return nil, errors.New("pose is not allowed to be nil")
	}
	if tf, ok := frame.(*translationalFrame); ok {
		return &staticFrame{tf.Name(), pose, tf.geometryCreator}, nil
	}
	if tf, ok := frame.(*staticFrame); ok {
		return &staticFrame{tf.Name(), pose, tf.geometryCreator}, nil
	}
	return &staticFrame{frame.Name(), pose, nil}, nil
}

// FrameFromPoint creates a new Frame from a 3D point.
func FrameFromPoint(name string, point r3.Vector) (Frame, error) {
	return NewStaticFrame(name, spatial.NewPoseFromPoint(point))
}

// Name is the name of the referenceframe.
func (sf *staticFrame) Name() string {
	return sf.name
}

// Transform returns the pose associated with this static referenceframe.
func (sf *staticFrame) Transform(inp []Input) (spatial.Pose, error) {
	if len(inp) != 0 {
		return nil, fmt.Errorf("given input length %q does not match frame DoF 0", len(inp))
	}
	return sf.transform, nil
}

// Geometries returns an object representing the 3D space associeted with the staticFrame.
func (sf *staticFrame) Geometries(input []Input) (map[string]spatial.Geometry, error) {
	if sf.geometryCreator == nil {
		return nil, fmt.Errorf("frame of type %T has nil geometryCreator", sf)
	}
	pose, err := sf.Transform(input)
	m := make(map[string]spatial.Geometry)
	m[sf.Name()] = sf.geometryCreator.NewGeometry(pose)
	return m, err
}

// DoF are the degrees of freedom of the transform. In the staticFrame, it is always 0.
func (sf *staticFrame) DoF() []Limit {
	return []Limit{}
}

func (sf *staticFrame) MarshalJSON() ([]byte, error) {
	transform, err := spatial.PoseMap(sf.transform)
	if err != nil {
		return nil, err
	}
	m := FrameMapConfig{
		"type":      "static",
		"name":      sf.name,
		"transform": transform,
	}
	return json.Marshal(m)
}

func (sf *staticFrame) AlmostEquals(otherFrame Frame) bool {
	other, ok := otherFrame.(*staticFrame)
	return ok && sf.name == other.name && spatial.PoseAlmostEqual(sf.transform, other.transform)
}

// a prismatic Frame is a frame that can translate without rotation in any/all of the X, Y, and Z directions.
type translationalFrame struct {
	name            string
	transAxis       r3.Vector
	limit           []Limit
	geometryCreator spatial.GeometryCreator
}

// NewTranslationalFrame creates a frame given a name and the axis in which to translate.
func NewTranslationalFrame(name string, axis r3.Vector, limit Limit) (Frame, error) {
	return NewTranslationalFrameWithGeometry(name, axis, limit, nil)
}

// NewTranslationalFrameWithGeometry creates a frame given a given a name and the axis in which to translate.
// It also has an associated geometryCreator representing the space that it occupies in 3D space.  Pose is not allowed to be nil.
func NewTranslationalFrameWithGeometry(name string, axis r3.Vector, limit Limit, geometryCreator spatial.GeometryCreator) (Frame, error) {
	if spatial.R3VectorAlmostEqual(r3.Vector{}, axis, 1e-8) {
		return nil, errors.New("cannot use zero vector as translation axis")
	}
	return &translationalFrame{name: name, transAxis: axis.Normalize(), limit: []Limit{limit}, geometryCreator: geometryCreator}, nil
}

// Name is the name of the frame.
func (pf *translationalFrame) Name() string {
	return pf.name
}

// Transform returns a pose translated by the amount specified in the inputs.
func (pf *translationalFrame) Transform(input []Input) (spatial.Pose, error) {
	var err error
	if len(input) != 1 {
		return nil, fmt.Errorf("given input length %d does not match frame DoF %d", len(input), 1)
	}

	// We allow out-of-bounds calculations, but will return a non-nil error
	if input[0].Value < pf.limit[0].Min || input[0].Value > pf.limit[0].Max {
		err = fmt.Errorf("%.5f input out of bounds %v", input[0].Value, pf.limit[0])
	}
	return spatial.NewPoseFromPoint(pf.transAxis.Mul(input[0].Value)), err
}

// Geometries returns an object representing the 3D space associeted with the translationalFrame.
func (pf *translationalFrame) Geometries(input []Input) (map[string]spatial.Geometry, error) {
	if pf.geometryCreator == nil {
		return nil, fmt.Errorf("frame of type %T has nil geometryCreator", pf)
	}
	pose, err := pf.Transform(input)
	m := make(map[string]spatial.Geometry)
	m[pf.Name()] = pf.geometryCreator.NewGeometry(pose)
	return m, err
}

// DoF are the degrees of freedom of the transform.
func (pf *translationalFrame) DoF() []Limit {
	return pf.limit
}

func (pf *translationalFrame) MarshalJSON() ([]byte, error) {
	m := FrameMapConfig{
		"type":      "translational",
		"name":      pf.name,
		"transAxis": pf.transAxis,
		"limit":     pf.limit,
	}
	return json.Marshal(m)
}

func (pf *translationalFrame) AlmostEquals(otherFrame Frame) bool {
	other, ok := otherFrame.(*translationalFrame)
	return ok && pf.name == other.name &&
		spatial.R3VectorAlmostEqual(pf.transAxis, other.transAxis, 1e-8) &&
		limitsAlmostEqual(pf.DoF(), other.DoF())
}

type rotationalFrame struct {
	name    string
	rotAxis r3.Vector
	limit   []Limit
}

// NewRotationalFrame creates a new rotationalFrame struct.
// A standard revolute joint will have 1 DoF.
func NewRotationalFrame(name string, axis spatial.R4AA, limit Limit) (Frame, error) {
	axis.Normalize()
	return &rotationalFrame{
		name:    name,
		rotAxis: r3.Vector{axis.RX, axis.RY, axis.RZ},
		limit:   []Limit{limit},
	}, nil
}

// Transform returns the Pose representing the frame's 6DoF motion in space. Requires a slice
// of inputs that has length equal to the degrees of freedom of the referenceframe.
func (rf *rotationalFrame) Transform(input []Input) (spatial.Pose, error) {
	var err error
	if len(input) != 1 {
		return nil, fmt.Errorf("given input length %d does not match frame DoF 1", len(input))
	}
	// We allow out-of-bounds calculations, but will return a non-nil error
	if input[0].Value < rf.limit[0].Min || input[0].Value > rf.limit[0].Max {
		err = fmt.Errorf("%.5f input out of rev frame bounds %.5f", input[0].Value, rf.limit[0])
	}
	// Create a copy of the r4aa for thread safety
	return spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, &spatial.R4AA{input[0].Value, rf.rotAxis.X, rf.rotAxis.Y, rf.rotAxis.Z}), err
}

// Geometries will always return (nil, nil) for rotationalFrames, as not allowing rotationalFrames to occupy geometries is a
// design choice made for simplicity. staticFrame and translationalFrame should be used instead.
func (rf *rotationalFrame) Geometries(input []Input) (map[string]spatial.Geometry, error) {
	return nil, fmt.Errorf("s not implemented for type %T", rf)
}

// DoF returns the number of degrees of freedom that a joint has. This would be 1 for a standard revolute joint.
func (rf *rotationalFrame) DoF() []Limit {
	return rf.limit
}

// Name returns the name of the referenceframe.
func (rf *rotationalFrame) Name() string {
	return rf.name
}

func (rf *rotationalFrame) MarshalJSON() ([]byte, error) {
	m := FrameMapConfig{
		"type":    "rotational",
		"name":    rf.name,
		"rotAxis": rf.rotAxis,
		"limit":   rf.limit,
	}
	return json.Marshal(m)
}

func (rf *rotationalFrame) AlmostEquals(otherFrame Frame) bool {
	other, ok := otherFrame.(*rotationalFrame)
	return ok && rf.name == other.name &&
		spatial.R3VectorAlmostEqual(rf.rotAxis, other.rotAxis, 1e-8) &&
		limitsAlmostEqual(rf.DoF(), other.DoF())
}
