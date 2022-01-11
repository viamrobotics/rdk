// Package referenceframe defines the api and does the math of translating between reference frames
// Useful for if you have a camera, connected to a gripper, connected to an arm,
// and need to translate the camera reference frame to the arm reference frame,
// if you've found something in the camera, and want to move the gripper + arm to get it.
package referenceframe

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"

	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"

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

	// Volumes returns a map between names and volumes for the reference frame and any intermediate frames that
	// may be defined for it, e.g. links in an arm. If a frame does not have a volumeCreator it will not be added into the map
	Volumes([]Input) (map[string]spatial.Volume, error)

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
	name          string
	transform     spatial.Pose
	volumeCreator spatial.VolumeCreator
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

// NewStaticFrameWithVolume creates a frame given a pose relative to its parent.  The pose is fixed for all time.
// It also has an associated volumeCreator representing the space that it occupies in 3D space.  Pose is not allowed to be nil.
func NewStaticFrameWithVolume(name string, pose spatial.Pose, volumeCreator spatial.VolumeCreator) (Frame, error) {
	if pose == nil {
		return nil, errors.New("pose is not allowed to be nil")
	}
	return &staticFrame{name, pose, volumeCreator}, nil
}

// NewStaticFrameFromFrame creates a frame given a pose relative to its parent.  The pose is fixed for all time.
// It inherits its name and volumeCreator properties from the specified Frame. Pose is not allowed to be nil.
func NewStaticFrameFromFrame(frame Frame, pose spatial.Pose) (Frame, error) {
	if pose == nil {
		return nil, errors.New("pose is not allowed to be nil")
	}
	if tf, ok := frame.(*translationalFrame); ok {
		return &staticFrame{tf.Name(), pose, tf.volumeCreator}, nil
	}
	if tf, ok := frame.(*staticFrame); ok {
		return &staticFrame{tf.Name(), pose, tf.volumeCreator}, nil
	}
	return &staticFrame{frame.Name(), pose, nil}, nil
}

// FrameFromPoint creates a new Frame from a 3D point.
func FrameFromPoint(name string, point r3.Vector) (Frame, error) {
	pose := spatial.NewPoseFromPoint(point)
	return NewStaticFrame(name, pose)
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

// Volumes returns an object representing the 3D space associeted with the staticFrame.
func (sf *staticFrame) Volumes(input []Input) (map[string]spatial.Volume, error) {
	if sf.volumeCreator == nil {
		return nil, fmt.Errorf("frame of type %T has nil volumeCreator", sf)
	}
	pose, err := sf.Transform(input)
	m := make(map[string]spatial.Volume)
	m[sf.Name()] = sf.volumeCreator.NewVolume(pose)
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
	m := map[string]interface{}{
		"type":      "static",
		"name":      sf.name,
		"transform": transform,
	}
	return json.Marshal(m)
}

func (sf *staticFrame) AlmostEquals(otherFrame Frame) bool {
	other, ok := otherFrame.(*staticFrame)
	if !ok {
		return false
	}
	return sf.name == other.name && spatial.PoseAlmostEqual(sf.transform, other.transform)
}

// a prismatic Frame is a frame that can translate without rotation in any/all of the X, Y, and Z directions.
type translationalFrame struct {
	name          string
	axes          []bool // if it moves along each axes, x, y, z
	limits        []Limit
	volumeCreator spatial.VolumeCreator
}

// NewTranslationalFrame creates a frame given a name and the axes in which to translate.
func NewTranslationalFrame(name string, axes []bool, limits []Limit) (Frame, error) {
	pf := &translationalFrame{name: name, axes: axes}
	if len(limits) != pf.DoFInt() {
		return nil, fmt.Errorf("given number of limits %d does not match number of axes %d", len(limits), pf.DoFInt())
	}
	pf.limits = limits
	return pf, nil
}

// NewTranslationalFrameWithVolume creates a frame given a given a name and the axes in which to translate.
// It also has an associated volumeCreator representing the space that it occupies in 3D space.  Pose is not allowed to be nil.
func NewTranslationalFrameWithVolume(name string, axes []bool, limits []Limit, volumeCreator spatial.VolumeCreator) (Frame, error) {
	pf := &translationalFrame{name: name, axes: axes, volumeCreator: volumeCreator}
	if len(limits) != pf.DoFInt() {
		return nil, fmt.Errorf("given number of limits %d does not match number of axes %d", len(limits), pf.DoFInt())
	}
	pf.limits = limits
	return pf, nil
}

// Name is the name of the frame.
func (pf *translationalFrame) Name() string {
	return pf.name
}

// Transform returns a pose translated by the amount specified in the inputs.
func (pf *translationalFrame) Transform(input []Input) (spatial.Pose, error) {
	var err error
	if len(input) != pf.DoFInt() {
		return nil, fmt.Errorf("given input length %d does not match frame DoF %d", len(input), pf.DoFInt())
	}
	translation := make([]float64, 3)
	tIdx := 0
	for i, v := range pf.axes {
		if v {
			// We allow out-of-bounds calculations, but will return a non-nil error
			if input[tIdx].Value < pf.limits[tIdx].Min || input[tIdx].Value > pf.limits[tIdx].Max {
				err = fmt.Errorf("%.5f input out of bounds %v", input[tIdx].Value, pf.limits[tIdx])
			}
			translation[i] = input[tIdx].Value
			tIdx++
		}
	}
	q := spatial.NewPoseFromPoint(r3.Vector{translation[0], translation[1], translation[2]})
	return q, err
}

// Volumes returns an object representing the 3D space associeted with the translationalFrame.
func (pf *translationalFrame) Volumes(input []Input) (map[string]spatial.Volume, error) {
	if pf.volumeCreator == nil {
		return nil, fmt.Errorf("frame of type %T has nil volumeCreator", pf)
	}
	pose, err := pf.Transform(input)
	m := make(map[string]spatial.Volume)
	m[pf.Name()] = pf.volumeCreator.NewVolume(pose)
	return m, err
}

// DoF are the degrees of freedom of the transform.
func (pf *translationalFrame) DoF() []Limit {
	return pf.limits
}

// DoFInt returns the quantity of axes in which this frame can translate.
func (pf *translationalFrame) DoFInt() int {
	DoF := 0
	for _, v := range pf.axes {
		if v {
			DoF++
		}
	}
	return DoF
}

func (pf *translationalFrame) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"type":   "translational",
		"name":   pf.name,
		"axes":   pf.axes,
		"limits": pf.limits,
	}
	return json.Marshal(m)
}

func (pf *translationalFrame) AlmostEquals(otherFrame Frame) bool {
	other, ok := otherFrame.(*translationalFrame)
	if !ok {
		return false
	}

	if pf.name != other.name {
		return false
	}

	// axes
	if len(pf.axes) != len(other.axes) {
		return false
	}

	for idx, a := range pf.axes {
		if a != other.axes[idx] {
			return false
		}
	}

	return limitsAlmostEqual(pf.limits, other.limits)
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
	rf := rotationalFrame{
		name:    name,
		rotAxis: r3.Vector{axis.RX, axis.RY, axis.RZ},
		limit:   []Limit{limit},
	}

	return &rf, nil
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

	pose := spatial.NewPoseFromOrientation(r3.Vector{0, 0, 0}, &spatial.R4AA{input[0].Value, rf.rotAxis.X, rf.rotAxis.Y, rf.rotAxis.Z})

	return pose, err
}

// Volumes will always return (nil, nil) for rotationalFrames, as not allowing rotationalFrames to occupy volumes is a
// design choice made for simplicity. staticFrame and translationalFrame should be used instead.
func (rf *rotationalFrame) Volumes(input []Input) (map[string]spatial.Volume, error) {
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
	m := map[string]interface{}{
		"type":    "rotational",
		"name":    rf.name,
		"rotAxis": rf.rotAxis,
		"limit":   rf.limit,
	}
	return json.Marshal(m)
}

func (rf *rotationalFrame) AlmostEquals(otherFrame Frame) bool {
	other, ok := otherFrame.(*rotationalFrame)
	if !ok {
		return false
	}

	const epsilon = 1e-5
	return rf.name == other.name &&
		limitsAlmostEqual(rf.limit, other.limit) &&
		utils.Float64AlmostEqual(rf.rotAxis.X, other.rotAxis.X, epsilon) &&
		utils.Float64AlmostEqual(rf.rotAxis.Y, other.rotAxis.Y, epsilon) &&
		utils.Float64AlmostEqual(rf.rotAxis.Z, other.rotAxis.Z, epsilon)
}

func decodePose(m map[string]interface{}) (spatial.Pose, error) {
	var point r3.Vector

	err := mapstructure.Decode(m["point"], &point)
	if err != nil {
		return nil, err
	}

	orientationMap, ok := m["orientation"].(map[string]interface{})
	if !ok {
		return nil, utils.NewUnexpectedTypeError(orientationMap, m["orientation"])
	}
	oType, ok := orientationMap["type"].(string)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(oType, orientationMap["type"])
	}
	oValue, ok := orientationMap["value"].(map[string]interface{})
	if !ok {
		return nil, utils.NewUnexpectedTypeError(oValue, orientationMap["value"])
	}
	jsonValue, err := json.Marshal(oValue)
	if err != nil {
		return nil, err
	}

	ro := spatial.RawOrientation{oType, jsonValue}
	orientation, err := spatial.ParseOrientation(ro)
	if err != nil {
		return nil, err
	}
	return spatial.NewPoseFromOrientation(point, orientation), nil
}

// UnmarshalFrameJSON deserialized json into a reference referenceframe.
func UnmarshalFrameJSON(data []byte) (Frame, error) {
	m := map[string]interface{}{}
	err := json.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}

	return UnmarshalFrameMap(m)
}

// UnmarshalFrameMap deserializes a Frame from a map.
func UnmarshalFrameMap(m map[string]interface{}) (Frame, error) {
	var err error

	switch m["type"] {
	case "static":
		f := staticFrame{}
		var ok bool
		f.name, ok = m["name"].(string)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(f.name, m["name"])
		}

		pose, ok := m["transform"].(map[string]interface{})
		if !ok {
			return nil, utils.NewUnexpectedTypeError(pose, m["transform"])
		}
		f.transform, err = decodePose(pose)
		if err != nil {
			return nil, fmt.Errorf("error decoding transform (%v) %w", m["transform"], err)
		}
		return &f, nil
	case "translational":
		f := translationalFrame{}
		var ok bool
		f.name, ok = m["name"].(string)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(f.name, m["name"])
		}
		err := mapstructure.Decode(m["axes"], &f.axes)
		if err != nil {
			return nil, err
		}
		err = mapstructure.Decode(m["limits"], &f.limits)
		if err != nil {
			return nil, err
		}
		return &f, nil
	case "rotational":
		f := rotationalFrame{}
		var ok bool
		f.name, ok = m["name"].(string)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(f.name, m["name"])
		}

		rotAxis, ok := m["rotAxis"].(map[string]interface{})
		if !ok {
			return nil, utils.NewUnexpectedTypeError(rotAxis, m["rotAxis"])
		}

		f.rotAxis.X, ok = rotAxis["X"].(float64)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(f.rotAxis.X, rotAxis["X"])
		}
		f.rotAxis.Y, ok = rotAxis["Y"].(float64)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(f.rotAxis.Y, rotAxis["Y"])
		}
		f.rotAxis.Z, ok = rotAxis["Z"].(float64)
		if !ok {
			return nil, utils.NewUnexpectedTypeError(f.rotAxis.Z, rotAxis["Z"])
		}

		err = mapstructure.Decode(m["limit"], &f.limit)
		if err != nil {
			return nil, err
		}
		return &f, nil

	default:
		return nil, fmt.Errorf("no frame type: [%v]", m["type"])
	}
}
