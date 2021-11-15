package referenceframe

import (
	"encoding/json"
	"math"
	"math/rand"

	"go.viam.com/core/spatialmath"

	"go.uber.org/multierr"
)

// ModelFramer has a method that returns the kinematics information needed to build a dynamic frame.
type ModelFramer interface {
	ModelFrame() *Model
}

// XYZWeights Defines a struct into which XYZ values can be parsed from JSON
type XYZWeights struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// XYZTHWeights Defines a struct into which XYZ + theta values can be parsed from JSON
type XYZTHWeights struct {
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	Z  float64 `json:"z"`
	TH float64 `json:"th"`
}

// SolverDistanceWeights values are used to augment the distance check for a given IK solution.
// For each component of a 6d pose, the distance from current position to goal is
// squared and then multiplied by the corresponding weight in this struct. The results
// are summed and that sum must be below a certain threshold.
// So values > 1 forces the IK algorithm to get that value closer to perfect than it
// otherwise would have, and values < 1 cause it to be more lax. A value of 0.0 will cause
// that dimension to not be considered at all.
type SolverDistanceWeights struct {
	Trans  XYZWeights   `json:"translation"`
	Orient XYZTHWeights `json:"orientation"`
}

// Model TODO
// Generally speaking, a Joint will attach a Body to a Frame
// And a Fixed will attach a Frame to a Body
// Exceptions are the head of the tree where we are just starting the robot from World
type Model struct {
	name string // the name of the arm
	// OrdTransforms is the list of transforms ordered from end effector to base
	OrdTransforms []Frame
	SolveWeights  SolverDistanceWeights
}

// NewModel constructs a new model.
func NewModel() *Model {
	m := Model{}
	m.SolveWeights = SolverDistanceWeights{XYZWeights{1.0, 1.0, 1.0}, XYZTHWeights{1.0, 1.0, 1.0, 1.0}}
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
	pos := make([]float64, len(inputs))
	for i, input := range inputs {
		pos[i] = input.Value
	}
	return m.JointRadToQuat(pos)
}

// JointRadToQuat takes a model and a list of joint angles in radians and computes the dual quaternion representing the
// cartesian position of the end effector. This is useful for when conversions between quaternions and OV are not needed.
func (m *Model) JointRadToQuat(radAngles []float64) (spatialmath.Pose, error) {
	poses, err := m.GetPoses(radAngles)
	if err != nil && poses == nil {
		return nil, err
	}
	// Start at ((1+0i+0j+0k)+(+0+0i+0j+0k)ϵ)
	transformations := spatialmath.NewZeroPose()
	for _, pose := range poses {
		transformations = spatialmath.Compose(transformations, pose)
	}
	return transformations, err
}

// GetPoses returns the list of Poses which, when multiplied together in order, will yield the
// Pose representing the 6d cartesian position of the end effector.
func (m *Model) GetPoses(pos []float64) ([]spatialmath.Pose, error) {
	quats := make([]spatialmath.Pose, len(m.OrdTransforms))
	var errAll error
	posIdx := 0
	// OrdTransforms is ordered from end effector -> base, so we reverse the list to get quaternions from the base outwards.
	for i := len(m.OrdTransforms) - 1; i >= 0; i-- {
		transform := m.OrdTransforms[i]

		dof := len(transform.DoF())
		input := make([]Input, dof)
		for j := 0; j < dof; j++ {
			input[j] = Input{pos[posIdx]}
			posIdx++
		}

		quat, err := transform.Transform(input)
		// Fail if inputs are incorrect and pose is nil, but allow querying out-of-bounds positions
		if err != nil && quat == nil {
			return nil, err
		}
		multierr.AppendInto(&errAll, err)
		quats[len(quats)-i-1] = quat

	}
	return quats, errAll
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
		"tolerances":           m.SolveWeights,
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

	if m.SolveWeights != other.SolveWeights {
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
