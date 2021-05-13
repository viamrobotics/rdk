package kinematics

import (
	"math"
	"math/rand"

	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

// TODO(pl): initial implementations of Joint methods are for Revolute joints. We will need to update once we have robots
// with non-revolute joints.

// TODO(pl): Maybe we want to make this an interface which different joint types implement
// TODO(pl): Give all these variables better names once I know what they all do. Or at least a detailed description

// Axis TODO
type Axis int

// TODO
const (
	Xaxis Axis = iota
	Yaxis
	Zaxis
)

// Joint TODO
type Joint struct {
	axes        []Axis
	dofPosition int
	dofVelocity int
	max         []float64
	min         []float64
	offset      []float64
	position    []float64
	positionD   []float64
	positionDD  []float64
	SpatialMat  *mgl64.MatMxN
	wraparound  []bool
	descriptor  graph.Edge
	transform   *Transform
}

// NewJoint TODO
func NewJoint(dPos, dVel int) *Joint {
	j := Joint{}
	j.dofPosition = dPos
	j.dofVelocity = dVel
	j.SpatialMat = mgl64.NewMatrix(6, dPos)
	j.SpatialMat.Zero(6, dPos)
	j.wraparound = make([]bool, dPos)
	j.offset = make([]float64, dPos)
	j.position = make([]float64, dPos)
	j.positionD = make([]float64, dVel)
	j.positionDD = make([]float64, dVel)
	j.transform = NewTransform()

	return &j
}

// Clip TODO
func (j *Joint) Clip(q []float64) {
	for i := 0; i < j.GetDofPosition(); i++ {
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

// RandomJointPositions TODO
func (j *Joint) RandomJointPositions(rnd *rand.Rand) []float64 {
	var positions []float64
	for i := 0; i < j.GetDofPosition(); i++ {
		jRange := math.Abs(j.max[i] - j.min[i])
		// Note that rand is unseeded and so will produce the same sequence of floats every time
		// However, since this will presumably happen at different positions to different joints, this shouldn't matter
		newPos := rnd.Float64()*jRange + j.min[i]
		positions = append(positions, newPos)
	}
	return positions
}

// Distance returns the L2 normalized difference between two equal length arrays
// TODO(pl): Maybe we want to enforce length requirements? Currently this is only used by things calling joints.getDofPosition()
func Distance(q1, q2 []float64) float64 {
	for i := 0; i < len(q1); i++ {
		q1[i] = q1[i] - q2[i]
	}
	// 2 is the L value returning a standard L2 Normalization
	return floats.Norm(q1, 2)
}

// ForwardPosition TODO
func (j *Joint) ForwardPosition() {
	j.transform.ForwardPosition()
}

// ForwardVelocity TODO
// Note that this currently only works for 1DOF revolute joints
// Will need to be updated for ball joints
func (j *Joint) ForwardVelocity() {

	axis := -1
	// Only one DOF should have nonzero velocity for standard revolute joints
	// If this is not the joint for which we are calculating the Jacobial, all positionD will be 0
	for i, v := range j.positionD {
		if v > 0 {
			axis = int(j.axes[i])
		}
	}
	velQuat := j.transform.t.Quat
	if axis >= 0 {
		velQuat = dualquat.Number{deriv(velQuat.Real)[axis], quat.Number{}}
	}

	j.transform.out.v = dualquat.Mul(j.transform.in.v, velQuat)
}

// GetDof TODO
// Note(erd): Get prefix should be removed
func (j *Joint) GetDof() int {
	return len(j.positionD)
}

// GetDofPosition TODO
// Note(erd): Get prefix should be removed
func (j *Joint) GetDofPosition() int {
	return len(j.position)
}

// GetPosition returns the joint's position in radians
// Note(erd): Get prefix should be removed
func (j *Joint) GetPosition() []float64 {
	return j.position
}

// GetMinimum TODO
// Note(erd): Get prefix should be removed
func (j *Joint) GetMinimum() []float64 {
	return j.min
}

// GetMaximum TODO
// Note(erd): Get prefix should be removed
func (j *Joint) GetMaximum() []float64 {
	return j.max
}

// SetName TODO
func (j *Joint) SetName(name string) {
	j.transform.name = name
}

// GetName TODO
// Note(erd): Get prefix should be removed
func (j *Joint) GetName() string {
	return j.transform.name
}

// SetEdgeDescriptor TODO
func (j *Joint) SetEdgeDescriptor(edge graph.Edge) {
	j.descriptor = edge
}

// GetEdgeDescriptor TODO
// Note(erd): Get prefix should be removed
func (j *Joint) GetEdgeDescriptor() graph.Edge {
	return j.descriptor
}

// SetIn TODO
func (j *Joint) SetIn(in *Frame) {
	j.transform.in = in
}

// GetIn TODO
// Note(erd): Get prefix should be removed
func (j *Joint) GetIn() *Frame {
	return j.transform.in
}

// SetOut TODO
func (j *Joint) SetOut(out *Frame) {
	j.transform.out = out
}

// GetOut TODO
// Note(erd): Get prefix should be removed
func (j *Joint) GetOut() *Frame {
	return j.transform.out
}

// GetRotationVector will return about which axes this joint will rotate and how much
// Should be normalized to [0,1] for each axis
// Note(erd): Get prefix should be removed
func (j *Joint) GetRotationVector() quat.Number {
	return quat.Number{Imag: j.SpatialMat.At(0, 0), Jmag: j.SpatialMat.At(1, 0), Kmag: j.SpatialMat.At(2, 0)}
}

// SetAxesFromSpatial will note the
func (j *Joint) SetAxesFromSpatial() {
	if j.SpatialMat.At(0, 0) > 0 {
		j.axes = append(j.axes, Xaxis)
	}
	if j.SpatialMat.At(1, 0) > 0 {
		j.axes = append(j.axes, Yaxis)
	}
	if j.SpatialMat.At(2, 0) > 0 {
		j.axes = append(j.axes, Zaxis)
	}
}

// PointAtZ returns the quat about which to rotate to point this joint's axis at Z
// We use mgl64 Quats for this, because they have the function conveniently built in
func (j *Joint) PointAtZ() dualquat.Number {
	zAxis := mgl64.Vec3{0, 0, 1}
	rotVec := mgl64.Vec3{j.SpatialMat.At(0, 0), j.SpatialMat.At(1, 0), j.SpatialMat.At(2, 0)}
	zGlQuat := mgl64.QuatBetweenVectors(rotVec, zAxis)
	return dualquat.Number{quat.Number{zGlQuat.W, zGlQuat.V.X(), zGlQuat.V.Y(), zGlQuat.V.Z()}, quat.Number{}}
}

// GetOperationalVelocity TODO
func (j *Joint) GetOperationalVelocity() dualquat.Number {
	return j.transform.out.v
}

// SetPosition will set the joint's position in RADIANS
func (j *Joint) SetPosition(pos []float64) {
	j.position = pos
	angle := pos[0] + j.offset[0]

	r1 := dualquat.Number{Real: j.GetRotationVector()}
	r1.Real = quat.Scale(math.Sin(angle/2)/quat.Abs(r1.Real), r1.Real)
	r1.Real.Real += math.Cos(angle / 2)

	j.transform.t.Quat = r1

}

// SetVelocity will set the joint's velocity
func (j *Joint) SetVelocity(vel []float64) {
	j.positionD = vel
}

// Clamp ensures that all values are between a given range
// In this case, it ensures that joint limits are not exceeded
func (j *Joint) Clamp(posvec []float64) []float64 {
	for i, v := range posvec {
		if j.wraparound[i] {
			// TODO(pl): Implement
		} else {
			if v < j.min[i] {
				// Not sure if mutating the list as I iterate over it is bad form
				// But this should be safe to do
				posvec[i] = j.min[i]
			} else if v > j.max[i] {
				posvec[i] = j.max[i]
			}
		}
	}
	return posvec
}

// Step TODO
// TODO(pl): This only will work when posvec and dpos are the same length
// Other joint types e.g. spherical will need to reimplement
func (j *Joint) Step(posvec, dpos []float64) []float64 {
	posvec2 := make([]float64, len(posvec))
	for i := range posvec {
		posvec2[i] = posvec[i] + dpos[i]
	}
	// Note- clamping should be disabled for now. We are better able to solve IK if the joints are mathematically
	// allowed to spin freely. Normalization and validity checking will prevent limits from being exceeded.
	// posvec2 = j.Clamp(posvec2)
	return posvec2
}

// Normalize TODO
// Only valid for revolute joints
// This should ensure that joint positions are the lowest reasonable value
// For example, rather than 375 degrees, it should be 15 degrees
func (j *Joint) Normalize(posvec []float64) []float64 {
	remain := math.Remainder(posvec[0], 2*math.Pi)
	if remain < j.min[0] {
		remain += 2 * math.Pi
	} else if remain > j.max[0] {
		remain -= 2 * math.Pi
	}
	return []float64{remain}
}

// IsValid TODO
func (j *Joint) IsValid(posvec []float64) bool {
	for i := range posvec {
		if posvec[i] < j.min[i] || posvec[i] > j.max[i] {
			return false
		}
	}
	return true
}
