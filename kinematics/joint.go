package kinematics

import (
	"math"
	"math/rand"

	//~ "fmt"

	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"

	//~ "go.viam.com/robotcore/kinematics"
	//~ "go.viam.com/robotcore/kinematics/kinmath/spatial"
)

// TODO(pl): initial implementations of Joint methods are for Revolute joints. We will need to update once we have robots
// with non-revolute joints.

// TODO(pl): Maybe we want to make this an interface which different joint types implement
// TODO(pl): Give all these variables better names once I know what they all do. Or at least a detailed description
type Joint struct {
	dofPosition int
	dofVelocity int
	max         []float64
	min         []float64
	offset      []float64
	position    []float64
	positionD   []float64
	positionDD  []float64
	SpatialMat  *mgl64.MatMxN
	//~ v           *spatial.MotionVector
	wraparound  []bool
	descriptor  graph.Edge
	transform   *Transform
}

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
	//~ j.v = &spatial.MotionVector{}
	//~ j.v.SetZero()

	return &j
}

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

// TODO(pl): Maybe we want to enforce length requirements? Currently this is only used by things calling joints.getDofPosition()
// Distance returns the L2 normalized difference between two equal length arrays
func Distance(q1, q2 []float64) float64 {
	for i := 0; i < len(q1); i++ {
		q1[i] = q1[i] - q2[i]
	}
	// 2 is the L value returning a standard L2 Normalization
	return floats.Norm(q1, 2)
}

func (j *Joint) ForwardPosition() {
	j.transform.ForwardPosition()
	//t.out.i.t.Quat = t.in.i.t.Transformation(t.t.Quat)
}

// This is currently written such that it works for Jacobians
// In other words it works as though this is the only moving joint
// DO NOT try to be able to this for dynamics in its present state
func (j *Joint) ForwardVelocity() {
	
	orientedModel := dualquat.Mul(j.transform.in.i.t.Quat, j.PointAtZ())
	
	//~ fmt.Println("q", orientedModel)
	j.transform.out.v = quatDeriv(orientedModel)
	
	//~ j.transform.out.v = j.transform.x.MultMV(j.transform.in.v)
	//~ j.transform.out.v.AddMV(j.v)
}

func (j *Joint) GetDof() int {
	return len(j.positionD)
}

func (j *Joint) GetDofPosition() int {
	return len(j.position)
}

// Returns the joint's position in radians
func (j *Joint) GetPosition() []float64 {
	return j.position
}

func (j *Joint) GetMinimum() []float64 {
	return j.min
}

func (j *Joint) GetMaximum() []float64 {
	return j.max
}

func (j *Joint) SetName(name string) {
	j.transform.name = name
}

func (j *Joint) GetName() string {
	return j.transform.name
}

func (j *Joint) SetEdgeDescriptor(edge graph.Edge) {
	j.descriptor = edge
}

func (j *Joint) GetEdgeDescriptor() graph.Edge {
	return j.descriptor
}

func (j *Joint) SetIn(in *Frame) {
	j.transform.in = in
}

func (j *Joint) GetIn() *Frame {
	return j.transform.in
}

func (j *Joint) SetOut(out *Frame) {
	j.transform.out = out
}

func (j *Joint) GetOut() *Frame {
	return j.transform.out
}

// GetRotationVector will return about which axes this joint will rotate and how much
// Should be normalized to [0,1] for each axis
func (j *Joint) GetRotationVector() quat.Number {
	return quat.Number{Imag: j.SpatialMat.At(0, 0), Jmag: j.SpatialMat.At(1, 0), Kmag: j.SpatialMat.At(2, 0)}
}

// PointAtZ returns the quat about which to rotate to point this joint's axis at Z
// We use mgl64 Quats for this, because they have the function conveniently built in
func (j *Joint) PointAtZ() dualquat.Number {
	zAxis := mgl64.Vec3{0,0,1}
	rotVec := mgl64.Vec3{j.SpatialMat.At(0, 0), j.SpatialMat.At(1, 0), j.SpatialMat.At(2, 0)}
	zGlQuat := mgl64.QuatBetweenVectors(rotVec, zAxis)
	return dualquat.Number{quat.Number{zGlQuat.W, zGlQuat.V.X(), zGlQuat.V.Y(), zGlQuat.V.Z()}, quat.Number{}}
}


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

	//~ j.transform.x.Rotation = j.transform.t.Linear().Transpose()
	//~ j.transform.x.Rotation = j.transform.t.Rotation()
}

// SetVelocity will set the joint's velocity
func (j *Joint) SetVelocity(vel []float64) {
	j.positionD = vel
	//~ motionVec := j.SpatialMat.MulNx1(mgl64.NewVecN(0), mgl64.NewVecNFromData(vel))
	//~ j.v = spatial.NewMVFromVecN(motionVec)
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

// TODO(pl): This only will work when posvec and dpos are the same length
// Other joint types e.g. spherical will need to reimplement
func (j *Joint) Step(posvec, dpos []float64) []float64 {
	posvec2 := make([]float64, len(posvec))
	for i := range posvec {
		posvec2[i] = posvec[i] + dpos[i]
	}
	//~ posvec2 = j.Clamp(posvec2)
	return posvec2
}

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

func (j *Joint) IsValid(posvec []float64) bool {
	for i := range posvec {
		if posvec[i] < j.min[i] || posvec[i] > j.max[i] {
			return false
		}
	}
	return true
}


// Given a quaternion representing FK up to a certain joint, this will calculate a quaternion which,
// if multiplied by the end effector's operational position, gives the velocity of the end effector at
// the various quaternion values.
// IMPORTANT: this assumes rotation around the Z axis. If your joint rotates around e.g. the Y axis, you
// must rotate qIn by 90 degrees aroung X, then call quatDeriv, then un-rotate.
func quatDeriv (qIn dualquat.Number) dualquat.Number{
	qReal := qIn.Real
	qDual := qIn.Dual
	dq := dualquat.Number{}
	
	dq.Real.Imag = qReal.Imag * qReal.Kmag + qReal.Real * qReal.Jmag
	dq.Real.Jmag = qReal.Jmag * qReal.Kmag - qReal.Real * qReal.Imag
	dq.Real.Kmag = (qReal.Kmag * qReal.Kmag - qReal.Jmag * qReal.Jmag - qReal.Imag * qReal.Imag + qReal.Real * + qReal.Real) / 2
	
	dq.Dual.Imag = qReal.Imag * qDual.Kmag + qDual.Imag * qReal.Kmag + qReal.Real * qDual.Jmag + qDual.Real * qReal.Jmag
	dq.Dual.Jmag = qReal.Jmag * qDual.Kmag + qDual.Jmag * qReal.Kmag - qReal.Real * qDual.Imag - qDual.Real * qReal.Imag
	dq.Dual.Kmag = qReal.Kmag * qDual.Kmag - qReal.Jmag * qDual.Jmag - qReal.Imag * qDual.Imag + qReal.Real * qDual.Real
	
	return dq
}
