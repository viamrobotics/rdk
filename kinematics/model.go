package kinematics

import (
	"math/rand"

	"go.viam.com/core/spatialmath"
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

// DistanceConfig values are used to augment the distance check for a given IK solution.
// For each component of a 6d pose, the distance from current position to goal is
// squared and then multiplied by the corresponding weight in this struct. The results
// are summed and that sum must be below a certain threshold.
// So values > 1 forces the IK algorithm to get that value closer to perfect than it
// otherwise would have, and values < 1 cause it to be more lax. A value of 0.0 will cause
// that dimension to not be considered at all.
type DistanceConfig struct {
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
	OrdTransforms []Transform
	DistCfg       DistanceConfig
}

// NewModel constructs a new model.
func NewModel() *Model {
	m := Model{}
	m.DistCfg = DistanceConfig{XYZWeights{1.0, 1.0, 1.0}, XYZTHWeights{1.0, 1.0, 1.0, 1.0}}
	return &m
}

// RandomJointPositions generates a list of radian joint positions that are random but valid for each joint.
func (m *Model) RandomJointPositions(randSeed *rand.Rand) []float64 {
	var jointPos []float64
	for _, joint := range m.Joints() {
		jointPos = append(jointPos, joint.RandomJointPositions(randSeed)...)
	}
	return jointPos
}

// Joints returns an array of all joints.
func (m *Model) Joints() []*Joint {
	var joints []*Joint
	for i := len(m.OrdTransforms) - 1; i >= 0; i-- {
		transform := m.OrdTransforms[i]
		if joint, ok := transform.(*Joint); ok {
			joints = append(joints, joint)
		}
	}
	return joints
}

// GetMinimum returns the array of GetPosition from all joints.
func (m *Model) GetMinimum() []float64 {
	var jointMin []float64
	for _, joint := range m.Joints() {
		jointMin = append(jointMin, joint.GetMinimum()...)
	}
	return jointMin
}

// GetMaximum returns the array of GetPosition from all joints.
func (m *Model) GetMaximum() []float64 {
	var jointMax []float64
	for _, joint := range m.Joints() {
		jointMax = append(jointMax, joint.GetMaximum()...)
	}
	return jointMax
}

// Normalize normalizes each of an array of joint positions- that is, enforces they are between +/- 2pi.
func (m *Model) Normalize(pos []float64) []float64 {
	i := 0
	var normalized []float64
	for _, joint := range m.Joints() {
		normalized = append(normalized, joint.Normalize(pos[i:i+joint.Dof()])...)
		i += joint.Dof()
	}
	return normalized
}

// GetQuaternions returns the list of DualQuaternions which, when multiplied together in order, will yield the
// dual quaternion representing the 6d cartesian position of the end effector.
func (m *Model) GetQuaternions(pos []float64) []*spatialmath.DualQuaternion {
	var quats []*spatialmath.DualQuaternion
	posIdx := 0
	for i := len(m.OrdTransforms) - 1; i >= 0; i-- {
		transform := m.OrdTransforms[i]
		quat := transform.Quaternion()
		if joint, ok := transform.(*Joint); ok {
			qDof := joint.Dof()
			quat = joint.AngleQuaternion(pos[posIdx : posIdx+qDof])
			posIdx += qDof
		}
		quats = append(quats, quat)

	}
	return quats
}

// IsValid checks whether the given array of joint positions violates any joint limits.
func (m *Model) IsValid(pos []float64) bool {
	i := 0
	for _, joint := range m.Joints() {
		if !(joint.IsValid(pos[i : i+joint.Dof()])) {
			return false
		}
		i += joint.Dof()
	}
	return true
}

// OperationalDof returns the number of end effectors. Currently we only support one end effector but will support more.
func (m *Model) OperationalDof() int {
	return 1
}

// Dof returns the number of degrees of freedom within an arm.
func (m *Model) Dof() int {
	numDof := 0
	for _, joint := range m.Joints() {
		numDof += joint.Dof()
	}
	return numDof
}
