package kinematics

import (
	"math"
	"math/rand"

	"go.viam.com/core/spatialmath"

	"gonum.org/v1/gonum/num/dualquat"
)

// TODO(pl): initial implementations of Joint methods are for Revolute joints. We will need to update once we have robots
// with non-revolute joints.

// TODO(pl): Maybe we want to make this an interface which different joint types implement
// TODO(pl): Give all these variables better names once I know what they all do. Or at least a detailed description

// Joint TODO
type Joint struct {
	parent     string
	rotAxis    spatialmath.R4AA
	dof        int
	max        []float64
	min        []float64
	wraparound []bool
}

// NewJoint creates a new Joint struct with the specified number of degrees of freedom.
// A standard revolute joint will have 1 DOF, a spherical joint will have 3.
func NewJoint(axis spatialmath.R4AA, parent string) *Joint {
	j := Joint{}
	j.parent = parent
	j.rotAxis = axis
	j.rotAxis.Normalize()

	// Currently we only support 1dof joints
	j.dof = 1
	j.wraparound = make([]bool, j.dof)

	return &j
}

// Clamp ensures that all values are between a given range.
// In this case, it ensures that joint limits are not exceeded.
func (j *Joint) Clamp(q []float64) {
	for i := 0; i < j.Dof(); i++ {
		if j.wraparound[i] {
			jRange := math.Abs(j.max[i] - j.min[i])
			for q[i] > j.max[i] {
				q[i] -= jRange
			}
			for q[i] < j.min[i] {
				q[i] += jRange
			}
		} else if q[i] > j.max[i] {
			q[i] = j.max[i]
		} else if q[i] < j.min[i] {
			q[i] = j.min[i]
		}
	}
}

// GenerateRandomJointPositions returns a list of random, guaranteed valid, positions for the joint.
func (j *Joint) GenerateRandomJointPositions(rnd *rand.Rand) []float64 {
	var positions []float64
	for i := 0; i < j.Dof(); i++ {
		jRange := math.Abs(j.max[i] - j.min[i])
		// Note that rand is unseeded and so will produce the same sequence of floats every time
		// However, since this will presumably happen at different positions to different joints, this shouldn't matter
		newPos := rnd.Float64()*jRange + j.min[i]
		positions = append(positions, newPos)
	}
	return positions
}

// Quaternion gets the quaternion representing this joint's rotation in space AT THE ZERO ANGLE.
func (j *Joint) Quaternion() *spatialmath.DualQuaternion {
	jointQuat := spatialmath.NewDualQuaternion()
	for i := 0; i < j.Dof(); i++ {
		rotation := j.rotAxis
		jointQuat.Quat = jointQuat.Transformation(dualquat.Number{Real: rotation.ToQuat()})
	}
	return jointQuat
}

// AngleQuaternion returns the quaternion representing this joint's rotation in space.
// If this is a joint with more than 1 DOF, it will return the quaternion representing the total rotation.
// Important math: this is the specific location where a joint radian is converted to a quaternion.
func (j *Joint) AngleQuaternion(angle []float64) *spatialmath.DualQuaternion {
	jQuat := spatialmath.NewDualQuaternion()
	for i := 0; i < j.Dof(); i++ {
		rotation := j.rotAxis
		rotation.Theta = angle[i]
		jQuat.Quat = jQuat.Transformation(dualquat.Number{Real: rotation.ToQuat()})
	}
	return jQuat
}

// Dof returns the number of degrees of freedom that a joint has. This would be 1 for a standard revolute joint, 3 for
// a spherical joint, etc.
func (j *Joint) Dof() int {
	return j.dof
}

// MinimumJointLimits returns the minimum allowable values for this joint.
func (j *Joint) MinimumJointLimits() []float64 {
	return j.min
}

// MaximumJointLimits returns the maximum allowable values for this joint.
func (j *Joint) MaximumJointLimits() []float64 {
	return j.max
}

// Normalize will ensure that joint positions are the lowest reasonable absolute value. If the provided joint position
// is outside the min/max for the joint, it will add/subtract 360 degrees to put it within that range.
// For example, rather than 375 degrees, it should be 15 degrees
func (j *Joint) Normalize(posvec []float64) []float64 {
	remain := make([]float64, j.Dof())
	for i := 0; i < j.Dof(); i++ {
		remain[i] = math.Remainder(posvec[i], 2*math.Pi)
		if remain[i] < j.min[i] {
			remain[i] += 2 * math.Pi
		} else if remain[i] > j.max[i] {
			remain[i] -= 2 * math.Pi
		}
	}
	return remain
}

// AreJointPositionsValid checks whether the provided joint position is within the min/max for the joint
func (j *Joint) AreJointPositionsValid(posvec []float64) bool {
	for i := range posvec {
		if posvec[i] < j.min[i] || posvec[i] > j.max[i] {
			return false
		}
	}
	return true
}

// Parent will return the name of the next transform up the kinematics chain from this joint.
func (j *Joint) Parent() string {
	return j.parent
}
