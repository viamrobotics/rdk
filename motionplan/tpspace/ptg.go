// Package tpspace defines an assortment of precomputable trajectories which can be used to plan nonholonomic 2d motion
package tpspace

import (
	"context"
	"math"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const floatEpsilon = 0.0001 // If floats are closer than this consider them equal

// PTG is a Parameterized Trajectory Generator, which defines how to map back and forth from cartesian space to TP-space
// PTG coordinates are specified in polar coordinates (alpha, d)
// One of these is needed for each sort of motion that can be done.
type PTG interface {
	// CToTP will return the (alpha, dist) TP-space coordinates whose corresponding relative pose minimizes the given function
	CToTP(context.Context, func(spatialmath.Pose) float64) (*TrajNode, error)

	// RefDistance returns the maximum distance that a single trajectory may travel
	RefDistance() float64

	// Returns the set of trajectory nodes along the given trajectory, out to the requested distance
	Trajectory(alpha, dist float64) ([]*TrajNode, error)
}

// PTGProvider is something able to provide a set of PTGs associsated with it. For example, a frame which precomputes
// a number of PTGs.
type PTGProvider interface {
	// PTGs returns the list of PTGs associated with this provider
	PTGs() []PTG
}

// PrecomputePTG is a precomputable PTG.
type PrecomputePTG interface {
	// PTGVelocities returns the linear and angular velocity at a specific point along a trajectory
	PTGVelocities(alpha, dist float64) (float64, float64, error)
	Transform([]referenceframe.Input) (spatialmath.Pose, error)
}

// TrajNode is a snapshot of a single point in time along a PTG trajectory, including the distance along that trajectory,
// the elapsed time along the trajectory, and the linear and angular velocity at that point.
type TrajNode struct {
	// TODO: cache pose point and orientation so that we don't recompute every time we need it
	Pose       spatialmath.Pose // for 2d, we only use x, y, and OV theta
	Time       float64          // elapsed time on trajectory
	Dist       float64          // distance travelled down trajectory
	Alpha      float64          // alpha k-value at this node
	LinVelMMPS float64          // linvel in millimeters per second at this node
	AngVelRPS  float64          // angvel in radians per second at this node

	ptX float64
	ptY float64
}

// discretized path to alpha.
func index2alpha(k, numPaths uint) float64 {
	if k >= numPaths {
		return math.NaN()
	}
	if numPaths == 0 {
		return math.NaN()
	}
	return math.Pi * (-1.0 + 2.0*(float64(k)+0.5)/float64(numPaths))
}

func alpha2index(alpha float64, numPaths uint) uint {
	alpha = wrapTo2Pi(alpha+math.Pi) - math.Pi
	idx := uint(math.Round(0.5 * (float64(numPaths)*(1.0+alpha/math.Pi) - 1.0)))
	return idx
}

// Returns a given angle in the [0, 2pi) range.
func wrapTo2Pi(theta float64) float64 {
	return theta - 2*math.Pi*math.Floor(theta/(2*math.Pi))
}

// ComputePTG will compute all nodes of simPTG at the requested alpha, out to the requested distance, at the specified diffT resolution.
func ComputePTG(
	simPTG PrecomputePTG,
	alpha, refDist, diffT float64,
) ([]*TrajNode, error) {
	// Initialize trajectory with an all-zero node
	alphaTraj := []*TrajNode{{Pose: spatialmath.NewZeroPose()}}

	var err error
	var t, v, w float64
	dist := math.Copysign(math.Abs(v)*diffT, refDist)

	// Step through each time point for this alpha
	for math.Abs(dist) < math.Abs(refDist) {
		t += diffT
		nextNode, err := ComputePTGNode(simPTG, alpha, dist, t)
		if err != nil {
			return nil, err
		}
		v = nextNode.LinVelMMPS
		w = nextNode.AngVelRPS

		// Update velocities of last node because the computed velocities at this node are what should be set after passing the last node
		alphaTraj[len(alphaTraj)-1].LinVelMMPS = v
		alphaTraj[len(alphaTraj)-1].AngVelRPS = w

		alphaTraj = append(alphaTraj, nextNode)
		dist += math.Copysign(math.Abs(v)*diffT, refDist)
	}

	// Add final node
	alphaTraj[len(alphaTraj)-1].LinVelMMPS = v
	alphaTraj[len(alphaTraj)-1].AngVelRPS = w
	pose, err := simPTG.Transform([]referenceframe.Input{{alpha}, {refDist}})
	if err != nil {
		return nil, err
	}
	tNode := &TrajNode{pose, t, refDist, alpha, v, w, pose.Point().X, pose.Point().Y}
	alphaTraj = append(alphaTraj, tNode)
	return alphaTraj, nil
}

// ComputePTGNode will return the TrajNode of the requested PTG, at the specified alpha and dist. The provided time is used
// to fill in the time field.
func ComputePTGNode(
	simPTG PrecomputePTG,
	alpha, dist, atT float64,
) (*TrajNode, error) {
	v, w, err := simPTG.PTGVelocities(alpha, dist)
	if err != nil {
		return nil, err
	}

	// ptgIK caches these, so this should be fast. If cacheing is removed or a different simPTG used, this could be slow.
	pose, err := simPTG.Transform([]referenceframe.Input{{alpha}, {dist}})
	if err != nil {
		return nil, err
	}
	return &TrajNode{pose, atT, dist, alpha, v, w, pose.Point().X, pose.Point().Y}, nil
}
