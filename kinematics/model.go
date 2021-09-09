package kinematics

import (
	"math"
	"math/rand"

	"go.viam.com/core/referenceframe"
	"go.viam.com/core/spatialmath"

	"go.uber.org/multierr"
)

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
	manufacturer string
	name         string // the name of the arm
	// OrdTransforms is the list of transforms ordered from end effector to base
	OrdTransforms []referenceframe.Frame
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
	limits := m.Dof()
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
func (m *Model) Joints() []referenceframe.Frame {
	joints := make([]referenceframe.Frame, 0, len(m.OrdTransforms)-1)
	// OrdTransforms is ordered from end effector -> base, so we reverse the list to get joints from the base outwards.
	for i := len(m.OrdTransforms) - 1; i >= 0; i-- {
		transform := m.OrdTransforms[i]
		if len(transform.Dof()) > 0 {
			joints = append(joints, transform)
		}
	}
	return joints
}

// Name returns the name of this model
func (m *Model) Name() string {
	return m.name
}

// SetName changes the name of this model
func (m *Model) SetName(name string) {
	m.name = name
}

// Transform takes a model and a list of joint angles in radians and computes the dual quaternion representing the
// cartesian position of the end effector. This is useful for when conversions between quaternions and OV are not needed.
func (m *Model) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
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
	var quats []spatialmath.Pose
	var errAll error
	posIdx := 0
	// OrdTransforms is ordered from end effector -> base, so we reverse the list to get quaternions from the base outwards.
	for i := len(m.OrdTransforms) - 1; i >= 0; i-- {
		transform := m.OrdTransforms[i]

		var input []referenceframe.Input
		dof := len(transform.Dof())
		for j := 0; j < dof; j++ {
			input = append(input, referenceframe.Input{pos[posIdx]})
			posIdx++
		}

		quat, err := transform.Transform(input)
		// Fail if inputs are incorrect and pose is nil, but allow querying out-of-bounds positions
		if err != nil && quat == nil {
			return nil, err
		}
		multierr.AppendInto(&errAll, err)
		quats = append(quats, quat)

	}
	return quats, errAll
}

// AreJointPositionsValid checks whether the given array of joint positions violates any joint limits.
func (m *Model) AreJointPositionsValid(pos []float64) bool {
	limits := m.Dof()
	for i := 0; i < len(limits); i++ {
		if pos[i] < limits[i].Min || pos[i] > limits[i].Max {
			return false
		}
	}
	return true
}

// OperationalDof returns the number of end effectors. Currently we only support one end effector but will support more.
func (m *Model) OperationalDof() int {
	return 1
}

// Dof returns the number of degrees of freedom within an arm.
func (m *Model) Dof() []referenceframe.Limit {
	limits := []referenceframe.Limit{}
	for _, joint := range m.Joints() {
		limits = append(limits, joint.Dof()...)
	}
	return limits
}

func limitsToArrays(limits []referenceframe.Limit) ([]float64, []float64) {
	var min, max []float64
	for _, limit := range limits {
		min = append(min, limit.Min)
		max = append(max, limit.Max)
	}
	return min, max
}
