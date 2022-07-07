package referenceframe

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"math/rand"
	"strings"
	"sync"

	"go.uber.org/multierr"

	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/spatialmath"
)

// ModelFramer has a method that returns the kinematics information needed to build a dynamic referenceframe.
type ModelFramer interface {
	ModelFrame() Model
}

// A Model represents a frame that can change its name.
type Model interface {
	Frame
	ChangeName(string)
}

// SimpleModel TODO.
type SimpleModel struct {
	name string // the name of the model
	// OrdTransforms is the list of transforms ordered from end effector to base
	OrdTransforms []Frame
	poseCache     *sync.Map
	limits        []Limit
	lock          sync.RWMutex
}

// NewSimpleModel constructs a new model.
func NewSimpleModel() *SimpleModel {
	m := &SimpleModel{}
	m.poseCache = &sync.Map{}
	return m
}

// GenerateRandomConfiguration generates a list of radian joint positions that are random but valid for each joint.
func GenerateRandomConfiguration(m Model, randSeed *rand.Rand) []float64 {
	limits := m.DoF()
	jointPos := make([]float64, 0, len(limits))

	for i := 0; i < len(limits); i++ {
		jRange := math.Abs(limits[i].Max - limits[i].Min)
		// Note that rand is unseeded and so will produce the same sequence of floats every time
		// However, since this will presumably happen at different positions to different joints, this shouldn't matter
		newPos := randSeed.Float64()*jRange + limits[i].Min
		jointPos = append(jointPos, newPos)
	}
	return jointPos
}

// Name returns the name of this model.
func (m *SimpleModel) Name() string {
	return m.name
}

// ChangeName changes the name of this model - necessary for building frame systems.
func (m *SimpleModel) ChangeName(name string) {
	m.name = name
}

// Transform takes a model and a list of joint angles in radians and computes the dual quaternion representing the
// cartesian position of the end effector. This is useful for when conversions between quaternions and OV are not needed.
func (m *SimpleModel) Transform(inputs []Input) (spatialmath.Pose, error) {
	frames, err := m.inputsToFrames(inputs, false)
	if err != nil && frames == nil {
		return nil, err
	}
	return frames[0].transform, err
}

// InputFromProtobuf converts pb.JointPosition to inputs.
func (m *SimpleModel) InputFromProtobuf(jp *pb.JointPositions) []Input {
	inputs := make([]Input, 0, len(jp.Degrees))
	posIdx := 0
	for _, transform := range m.OrdTransforms {
		dof := len(transform.DoF()) + posIdx
		jPos := jp.Degrees[posIdx:dof]
		posIdx = dof

		inputs = append(inputs, transform.InputFromProtobuf(&pb.JointPositions{Degrees: jPos})...)
	}

	return inputs
}

// ProtobufFromInput converts inputs to pb.JointPosition.
func (m *SimpleModel) ProtobufFromInput(input []Input) *pb.JointPositions {
	jPos := &pb.JointPositions{}
	posIdx := 0
	for _, transform := range m.OrdTransforms {
		dof := len(transform.DoF()) + posIdx
		jPos.Degrees = append(jPos.Degrees, transform.ProtobufFromInput(input[posIdx:dof]).Degrees...)
		posIdx = dof
	}

	return jPos
}

// Geometries returns an object representing the 3D space associeted with the staticFrame.
func (m *SimpleModel) Geometries(inputs []Input) (map[string]spatialmath.Geometry, error) {
	frames, err := m.inputsToFrames(inputs, true)
	if err != nil && frames == nil {
		return nil, err
	}
	var errAll error
	geometryMap := make(map[string]spatialmath.Geometry)
	for _, frame := range frames {
		geometry, err := frame.Geometries([]Input{})
		if geometry == nil {
			// only propagate errors that result in nil geometry
			multierr.AppendInto(&errAll, err)
			continue
		}
		geometryMap[m.name+":"+frame.Name()] = geometry[frame.Name()]
	}
	return geometryMap, errAll
}

// CachedTransform will check a sync.Map cache to see if the exact given set of inputs has been computed yet. If so
// it returns without redoing the calculation. Thread safe, but so far has tended to be slightly slower than just doing
// the calculation. This may change with higher DOF models and longer runtimes.
func (m *SimpleModel) CachedTransform(inputs []Input) (spatialmath.Pose, error) {
	key := floatsToString(inputs)
	if val, ok := m.poseCache.Load(key); ok {
		if pose, ok := val.(spatialmath.Pose); ok {
			return pose, nil
		}
	}
	poses, err := m.inputsToFrames(inputs, false)
	if err != nil && poses == nil {
		return nil, err
	}
	m.poseCache.Store(key, poses[len(poses)-1].transform)

	return poses[len(poses)-1].transform, err
}

// IsConfigurationValid checks whether the given array of joint positions violates any joint limits.
func IsConfigurationValid(m Model, configuration []float64) bool {
	limits := m.DoF()
	for i := 0; i < len(limits); i++ {
		if configuration[i] < limits[i].Min || configuration[i] > limits[i].Max {
			return false
		}
	}
	return true
}

// OperationalDoF returns the number of end effectors. Currently we only support one end effector but will support more.
func (m *SimpleModel) OperationalDoF() int {
	return 1
}

// DoF returns the number of degrees of freedom within a model.
func (m *SimpleModel) DoF() []Limit {
	m.lock.RLock()
	if len(m.limits) > 0 {
		return m.limits
	}
	m.lock.RUnlock()

	limits := make([]Limit, 0, len(m.OrdTransforms)-1)
	for _, transform := range m.OrdTransforms {
		if len(transform.DoF()) > 0 {
			limits = append(limits, transform.DoF()...)
		}
	}
	m.lock.Lock()
	m.limits = limits
	m.lock.Unlock()
	return limits
}

// MarshalJSON serializes a Model.
func (m *SimpleModel) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"name":                 m.name,
		"kinematic_param_type": "frames",
		"frames":               m.OrdTransforms,
	})
}

// AlmostEquals returns true if the only difference between this model and another is floating point inprecision.
func (m *SimpleModel) AlmostEquals(otherFrame Frame) bool {
	other, ok := otherFrame.(*SimpleModel)
	if !ok {
		return false
	}

	if m.name != other.name {
		return false
	}

	if len(m.OrdTransforms) != len(other.OrdTransforms) {
		return false
	}

	for idx, f := range m.OrdTransforms {
		if !f.AlmostEquals(other.OrdTransforms[idx]) {
			return false
		}
	}

	return true
}

// TODO(rb) better comment
// takes a model and a list of joint angles in radians and computes the dual quaternion representing the
// cartesian position of each of the links up to and including the end effector. This is useful for when conversions
// between quaternions and OV are not needed.
func (m *SimpleModel) inputsToFrames(inputs []Input, collectAll bool) ([]*staticFrame, error) {
	var err error
	poses := make([]*staticFrame, 0, len(m.OrdTransforms))
	// Start at ((1+0i+0j+0k)+(+0+0i+0j+0k)ϵ)
	composedTransformation := spatialmath.NewZeroPose()
	posIdx := 0
	// get quaternions from the base outwards.
	for _, transform := range m.OrdTransforms {
		dof := len(transform.DoF()) + posIdx
		input := inputs[posIdx:dof]
		posIdx = dof

		pose, errNew := transform.Transform(input)
		// Fail if inputs are incorrect and pose is nil, but allow querying out-of-bounds positions
		if pose == nil || (err != nil && !strings.Contains(err.Error(), OOBErrString)) {
			return nil, err
		}
		multierr.AppendInto(&err, errNew)
		if collectAll {
			tf, err := NewStaticFrameFromFrame(transform, composedTransformation)
			if err != nil {
				return nil, err
			}
			poses = append(poses, tf.(*staticFrame))
		}
		composedTransformation = spatialmath.Compose(composedTransformation, pose)
	}
	// TODO(rb) as written this will return one too many frames, no need to return zeroth frame
	poses = append(poses, &staticFrame{"", composedTransformation, nil})
	return poses, err
}

// floatsToString turns a float array into a serializable binary representation
// This is very fast, about 100ns per call.
func floatsToString(inputs []Input) string {
	b := make([]byte, len(inputs)*8)
	for i, input := range inputs {
		binary.BigEndian.PutUint64(b[8*i:8*i+8], math.Float64bits(input.Value))
	}
	return string(b)
}
