package kinematics

import (
	"math"
	"math/rand"

	//~ "fmt"

	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/graph"

	//~ "github.com/viamrobotics/robotcore/kinematics"
	"github.com/viamrobotics/robotcore/kinematics/kinmath/spatial"
)

// TODO: initial implementations of Joint methods are for Revolute joints. We will need to update once we have robots
// with non-revolute joints.

// TODO: Maybe we want to make this an interface which different joint types implement
// TODO: Give all these variables better names once I know what they all do. Or at least a detailed description
type Joint struct {
	dofPosition int
	dofVelocity int
	//~ a           spatial.MotionVector
	//~ c           spatial.MotionVector
	//~ D           *mgl64.MatMxN
	max      []float64
	min      []float64
	offset   []float64
	position []float64
	//~ qUnits     []float64
	positionD []float64
	//~ qdUnits    []float64
	positionDD []float64
	//~ qddUnits   []float64
	SpatialMat *mgl64.MatMxN
	//~ speed       []float64
	//~ speedUnits  []float64
	//~ tau         []float64
	//~ tauUnits    []float64
	//~ u           []float64
	//~ U           *mgl64.MatMxN
	v          *spatial.MotionVector
	wraparound []bool
	descriptor graph.Edge
	transform  *Transform
}

func NewJoint(dPos, dVel int) *Joint {
	j := Joint{}
	j.dofPosition = dPos
	j.dofVelocity = dVel
	j.SpatialMat = mgl64.NewMatrix(6, dPos)
	j.SpatialMat.Zero(6, dPos)
	//~ j.U = mgl64.NewMatrix(6, dVel)
	//~ j.U.Zero(6, dVel)
	//~ j.a.SetZero()
	//~ j.c.SetZero()
	j.wraparound = make([]bool, dPos)
	j.offset = make([]float64, dPos)
	j.position = make([]float64, dPos)
	j.positionD = make([]float64, dVel)
	j.positionDD = make([]float64, dVel)
	j.transform = NewTransform()
	j.v = &spatial.MotionVector{}
	j.v.SetZero()

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

func (j *Joint) RandomJointPositions() []float64 {
	var positions []float64
	for i := 0; i < j.GetDofPosition(); i++ {
		jRange := math.Abs(j.max[i] - j.min[i])
		// Note that rand is unseeded and so will produce the same sequence of floats every time
		// However, since this will presumably happen at different positions to different joints, this shouldn't matter
		newPos := rand.Float64()*jRange + j.min[i]
		positions = append(positions, newPos)
	}
	return positions
}

// TODO: Maybe we want to enforce length requirements? Currently this is only used by things calling joints.getDofPosition()
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
}

func (j *Joint) ForwardVelocity() {
	j.transform.out.v = j.transform.x.MultMV(j.transform.in.v)
	j.transform.out.v.AddMV(j.v)
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
// So, returns a 3-element slice representing rotation around x,y,z axes
func (j *Joint) GetRotationVector() mgl64.Vec3 {
	return mgl64.Vec3{j.SpatialMat.At(0, 0), j.SpatialMat.At(1, 0), j.SpatialMat.At(2, 0)}
}

// SetPosition will set the joint's position in RADIANS
func (j *Joint) SetPosition(pos []float64) {
	j.position = pos
	angle := pos[0] + j.offset[0]

	j.transform.t.SetMatrix(mgl64.HomogRotate3D(angle, j.GetRotationVector()))
	//~ j.transform.x.Rotation = j.transform.t.Linear().Transpose()
	j.transform.x.Rotation = j.transform.t.Linear()
}

// SetVelocity will set the joint's velocity
func (j *Joint) SetVelocity(vel []float64) {
	j.positionD = vel
	motionVec := j.SpatialMat.MulNx1(mgl64.NewVecN(0), mgl64.NewVecNFromData(vel))
	j.v = spatial.NewMVFromVecN(motionVec)
}

// Clamp ensures that all values are between a given range
// In this case, it ensures that joint limits are not exceeded
func (j *Joint) Clamp(posvec []float64) []float64 {
	for i, v := range posvec {
		if j.wraparound[i] {
			// TODO: Implement
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

// TODO: This only will work when posvec and dpos are the same length
// Other joint types e.g. spherical will need to reimplement
func (j *Joint) Step(posvec, dpos []float64) []float64 {
	posvec2 := make([]float64, len(posvec))
	for i := range posvec {
		posvec2[i] = posvec[i] + dpos[i]
	}
	posvec2 = j.Clamp(posvec2)
	return posvec2
}

// Only valid for revolute joints
// This should ensure that joint positions are the lowest reasonable value
// For example, rather than 375 degrees, it should be 15 degrees
func (j *Joint) Normalize(posvec []float64) []float64 {
	remain := math.Remainder(posvec[0], math.Pi)
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
