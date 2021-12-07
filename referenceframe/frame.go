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

	spatial "go.viam.com/core/spatialmath"

	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
)

// Limit represents the limits of motion for a frame
type Limit struct {
	Min float64
	Max float64
}

func limitsALmostTheSame(a, b []Limit) bool {
	if len(a) != len(b) {
		return false
	}

	for idx, x := range a {
		if !float64AlmostEqual(x.Min, b[idx].Min) ||
			!float64AlmostEqual(x.Max, b[idx].Max) {
			return false
		}
	}

	return true
}

// RestrictedRandomFrameInputs will produce a list of valid, in-bounds inputs for the frame, restricting the range to
// `lim` percent of the limits
func RestrictedRandomFrameInputs(m Frame, rSeed *rand.Rand, lim float64) []Input {
	if rSeed == nil {
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

// RandomFrameInputs will produce a list of valid, in-bounds inputs for the frame
func RandomFrameInputs(m Frame, rSeed *rand.Rand) []Input {
	if rSeed == nil {
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
	// Name returns the name of the frame.
	Name() string

	// Transform is the pose (rotation and translation) that goes FROM current frame TO parent's frame.
	Transform([]Input) (spatial.Pose, error)

	// VerboseTransform returns a map between names and poses for the reference frame and any intermediate frames that
	// may be defined for it, e.g. links in an arm
	VerboseTransform([]Input) (map[string]spatial.Pose, error)

	// DoF will return a slice with length equal to the number of joints/degrees of freedom.
	// Each element describes the min and max movement limit of that joint/degree of freedom.
	// For robot parts that don't move, it returns an empty slice.
	DoF() []Limit

	// AlmostEquals returns if the otherFrame is close to the frame.
	// differences should just be things like floating point inprecision
	AlmostEquals(otherFrame Frame) bool

	json.Marshaler
}

// a static Frame is a simple corrdinate system that encodes a fixed translation and rotation from the current Frame to the parent Frame
type staticFrame struct {
	name      string
	transform spatial.Pose
}

// NewStaticFrame creates a frame given a pose relative to its parent. The pose is fixed for all time.
// Pose is not allowed to be nil.
func NewStaticFrame(name string, pose spatial.Pose) (Frame, error) {
	if pose == nil {
		return nil, errors.New("pose is not allowed to be nil")
	}
	return &staticFrame{name, pose}, nil
}

// NewZeroStaticFrame creates a frame with no translation or orientation changes
func NewZeroStaticFrame(name string) Frame {
	return &staticFrame{name, spatial.NewZeroPose()}
}

// FrameFromPoint creates a new Frame from a 3D point.
func FrameFromPoint(name string, point r3.Vector) (Frame, error) {
	pose := spatial.NewPoseFromPoint(point)
	return NewStaticFrame(name, pose)
}

// Name is the name of the frame.
func (sf *staticFrame) Name() string {
	return sf.name
}

// Transform returns the pose associated with this static frame.
func (sf *staticFrame) Transform(inp []Input) (spatial.Pose, error) {
	if len(inp) != 0 {
		return nil, fmt.Errorf("given input length %q does not match frame DoF 0", len(inp))
	}
	return sf.transform, nil
}

func (sf *staticFrame) VerboseTransform(input []Input) (map[string]spatial.Pose, error) {
	pose, err := sf.Transform(input)
	m := make(map[string]spatial.Pose)
	m[sf.Name()] = pose
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

func float64AlmostEqual(a, b float64) bool {
	return math.Abs(a-b) < .00001
}
func (sf *staticFrame) AlmostEquals(otherFrame Frame) bool {
	other, ok := otherFrame.(*staticFrame)
	if !ok {
		return false
	}

	return sf.name == other.name &&
		float64AlmostEqual(sf.transform.Point().X, other.transform.Point().X) &&
		float64AlmostEqual(sf.transform.Point().Y, other.transform.Point().Y) &&
		float64AlmostEqual(sf.transform.Point().Z, other.transform.Point().Z) &&
		float64AlmostEqual(sf.transform.Orientation().AxisAngles().RX, other.transform.Orientation().AxisAngles().RX) &&
		float64AlmostEqual(sf.transform.Orientation().AxisAngles().RY, other.transform.Orientation().AxisAngles().RY) &&
		float64AlmostEqual(sf.transform.Orientation().AxisAngles().RZ, other.transform.Orientation().AxisAngles().RZ) &&
		float64AlmostEqual(sf.transform.Orientation().AxisAngles().Theta, other.transform.Orientation().AxisAngles().Theta)
}

// a prismatic Frame is a frame that can translate without rotation in any/all of the X, Y, and Z directions
type translationalFrame struct {
	name   string
	axes   []bool // if it moves along each axes, x, y, z
	limits []Limit
}

// NewTranslationalFrame creates a frame given a name and the axes in which to translate
func NewTranslationalFrame(name string, axes []bool, limits []Limit) (Frame, error) {
	pf := &translationalFrame{name: name, axes: axes}
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

func (pf *translationalFrame) VerboseTransform(input []Input) (map[string]spatial.Pose, error) {
	pose, err := pf.Transform(input)
	m := make(map[string]spatial.Pose)
	m[pf.Name()] = pose
	return m, err
}

// DoF are the degrees of freedom of the transform.
func (pf *translationalFrame) DoF() []Limit {
	return pf.limits
}

// DoFInt returns the quantity of axes in which this frame can translate
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

	return limitsALmostTheSame(pf.limits, other.limits)
}

type rotationalFrame struct {
	name    string
	rotAxis r3.Vector
	limit   []Limit
}

// NewRotationalFrame creates a new rotationalFrame struct.
// A standard revolute joint will have 1 DoF
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
// of inputs that has length equal to the degrees of freedom of the frame.
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

func (rf *rotationalFrame) VerboseTransform(input []Input) (map[string]spatial.Pose, error) {
	pose, err := rf.Transform(input)
	m := make(map[string]spatial.Pose)
	m[rf.Name()] = pose
	return m, err
}

// DoF returns the number of degrees of freedom that a joint has. This would be 1 for a standard revolute joint.
func (rf *rotationalFrame) DoF() []Limit {
	return rf.limit
}

// Name returns the name of the frame
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

	return rf.name == other.name &&
		limitsALmostTheSame(rf.limit, other.limit) &&
		float64AlmostEqual(rf.rotAxis.X, other.rotAxis.X) &&
		float64AlmostEqual(rf.rotAxis.Y, other.rotAxis.Y) &&
		float64AlmostEqual(rf.rotAxis.Z, other.rotAxis.Z)
}

func decodePose(m map[string]interface{}) (spatial.Pose, error) {
	var point r3.Vector

	err := mapstructure.Decode(m["point"], &point)
	if err != nil {
		return nil, err
	}

	orientationMap := m["orientation"].(map[string]interface{})
	oType := orientationMap["type"].(string)
	oValue := orientationMap["value"].(map[string]interface{})
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

// UnmarshalFrameJSON deserialized json into a reference frame
func UnmarshalFrameJSON(data []byte) (Frame, error) {

	m := map[string]interface{}{}
	err := json.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}

	return UnmarshalFrameMap(m)
}

// UnmarshalFrameMap deserializes a Frame from a map
func UnmarshalFrameMap(m map[string]interface{}) (Frame, error) {
	var err error

	switch m["type"] {
	case "static":
		f := staticFrame{}
		f.name = m["name"].(string)

		pose := m["transform"].(map[string]interface{})
		f.transform, err = decodePose(pose)
		if err != nil {
			return nil, fmt.Errorf("error decoding transform (%v) %w", m["transform"], err)
		}
		return &f, nil
	case "translational":
		f := translationalFrame{}
		f.name = m["name"].(string)
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
		f.name = m["name"].(string)

		f.rotAxis.X = m["rotAxis"].(map[string]interface{})["X"].(float64)
		f.rotAxis.Y = m["rotAxis"].(map[string]interface{})["Y"].(float64)
		f.rotAxis.Z = m["rotAxis"].(map[string]interface{})["Z"].(float64)

		err = mapstructure.Decode(m["limit"], &f.limit)
		if err != nil {
			return nil, err
		}
		return &f, nil

	default:
		return nil, fmt.Errorf("no frame type: [%v]", m["type"])
	}

}
