package referenceframe

import (
	"encoding/json"
	"math"
	"math/rand"

	"go.viam.com/core/spatialmath"
)

// ModelFramer has a method that returns the kinematics information needed to build a dynamic frame.
type ModelFramer interface {
	ModelFrame() *Model
}

// Model TODO
// Generally speaking, a Joint will attach a Body to a Frame
// And a Fixed will attach a Frame to a Body
// Exceptions are the head of the tree where we are just starting the robot from World
type Model struct {
	name string // the name of the arm
	// OrdTransforms is the list of transforms ordered from end effector to base
	OrdTransforms []Frame
}

// NewModel constructs a new model.
func NewModel() *Model {
	m := Model{}
	return &m
}

// GenerateRandomJointPositions generates a list of radian joint positions that are random but valid for each joint.
func (m *Model) GenerateRandomJointPositions(randSeed *rand.Rand) []float64 {
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

// Joints returns an array of all settable frames in the model, from the base outwards.
func (m *Model) Joints() []Frame {
	joints := make([]Frame, 0, len(m.OrdTransforms)-1)
	// OrdTransforms is ordered from end effector -> base, so we reverse the list to get joints from the base outwards.
	for i := len(m.OrdTransforms) - 1; i >= 0; i-- {
		transform := m.OrdTransforms[i]
		if len(transform.DoF()) > 0 {
			joints = append(joints, transform)
		}
	}
	return joints
}

// Name returns the name of this model
func (m *Model) Name() string {
	return m.name
}

// ChangeName changes the name of this model - necessary for building frame systems
func (m *Model) ChangeName(name string) {
	m.name = name
}

// Transform takes a model and a list of joint angles in radians and computes the dual quaternion representing the
// cartesian position of the end effector. This is useful for when conversions between quaternions and OV are not needed.
func (m *Model) Transform(inputs []Input) (spatialmath.Pose, error) {
	poses, err := m.jointRadToQuats(inputs)
	if err != nil && poses == nil {
		return nil, err
	}
	return poses[len(poses)-1].transform, err
}

// VerboseTransform takes a model and a list of joint angles in radians and computes the dual quaterions representing
// the pose of each of the intermediate frames (if any exist) up to and including the end effector, and returns a map
// of frame names to poses. The key for each frame in the map will be the string "<model_name>:<frame_name>"
func (m *Model) VerboseTransform(inputs []Input) (map[string]spatialmath.Pose, error) {
	poses, err := m.jointRadToQuats(inputs)
	if err != nil && poses == nil {
		return nil, err
	}
	poseMap := make(map[string]spatialmath.Pose)
	for _, pose := range poses {
		poseMap[m.name+":"+pose.name] = pose.transform
	}
	return poseMap, err
}

// jointRadToQuats takes a model and a list of joint angles in radians and computes the dual quaternion representing the
// cartesian position of each of the links up to and including the end effector. This is useful for when conversions
// between quaternions and OV are not needed.
func (m *Model) jointRadToQuats(inputs []Input) ([]*staticFrame, error) {
	poses := make([]*staticFrame, 0, len(m.OrdTransforms))
	// Start at ((1+0i+0j+0k)+(+0+0i+0j+0k)ϵ)
	composedTransformation := spatialmath.NewZeroPose()
	posIdx := 0
	// OrdTransforms is ordered from end effector -> base, so we reverse the list to get quaternions from the base outwards.
	for i := len(m.OrdTransforms) - 1; i >= 0; i-- {
		transform := m.OrdTransforms[i]

		dof := len(transform.DoF())
		input := make([]Input, 0, dof)
		for j := 0; j < dof; j++ {
			input = append(input, inputs[posIdx])
			posIdx++
		}

		pose, err := transform.Transform(input)
		// Fail if inputs are incorrect and pose is nil, but allow querying out-of-bounds positions
		if err != nil && pose == nil {
			return nil, err
		}
		composedTransformation = spatialmath.Compose(composedTransformation, pose)
		poses = append(poses, &staticFrame{transform.Name(), composedTransformation})
	}
	return poses, nil
}

// AreJointPositionsValid checks whether the given array of joint positions violates any joint limits.
func (m *Model) AreJointPositionsValid(pos []float64) bool {
	limits := m.DoF()
	for i := 0; i < len(limits); i++ {
		if pos[i] < limits[i].Min || pos[i] > limits[i].Max {
			return false
		}
	}
	return true
}

// OperationalDoF returns the number of end effectors. Currently we only support one end effector but will support more.
func (m *Model) OperationalDoF() int {
	return 1
}

// DoF returns the number of degrees of freedom within an arm.
func (m *Model) DoF() []Limit {
	limits := []Limit{}
	for _, joint := range m.Joints() {
		limits = append(limits, joint.DoF()...)
	}
	return limits
}

// MarshalJSON serializes a Model
func (m *Model) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"name":                 m.name,
		"kinematic_param_type": "frames",
		"frames":               m.OrdTransforms,
	})
}

// AlmostEquals returns true if the only difference between this model and another is floating point inprecision
func (m *Model) AlmostEquals(otherFrame Frame) bool {
	other, ok := otherFrame.(*Model)
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
