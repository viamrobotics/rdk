// Package tpspace defines an assortment of precomputable trajectories which can be used to plan nonholonomic 2d motion
package tpspace

import (
	"math"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const floatEpsilon = 0.0001 // If floats are closer than this consider them equal
var flipPose = spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 180})

// PTGSolver wraps a PTG with the ability to perform Inverse Kinematics.
type PTGSolver interface {
	// Solve will return the (alpha, dist) TP-space coordinates whose corresponding relative pose minimizes the given function
	ik.InverseKinematics
	PTG

	// MaxDistance returns the maximum distance that a single trajectory may travel
	MaxDistance() float64

	// Returns the set of trajectory nodes along the given trajectory, out to the requested distance
	Trajectory(alpha, dist float64) ([]*TrajNode, error)
}

// PTGProvider is something able to provide a set of PTGs associsated with it. For example, a frame which precomputes
// a number of PTGs.
type PTGProvider interface {
	// PTGs returns the list of PTGs associated with this provider
	PTGSolvers() []PTGSolver
}

// PTG is a Parameterized Trajectory Generator, which defines how to map back and forth from cartesian space to TP-space
// PTG coordinates are specified in polar coordinates (alpha, d)
// One of these is needed for each sort of motion that can be done.
type PTG interface {
	// Velocities returns the linear and angular velocity at a specific point along a trajectory
	Velocities(alpha, dist float64) (float64, float64, error)
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

// Returns a given angle in the [0, 2pi) range.
func wrapTo2Pi(theta float64) float64 {
	return theta - 2*math.Pi*math.Floor(theta/(2*math.Pi))
}

// ComputePTG will compute all nodes of simPTG at the requested alpha, out to the requested distance, at the specified diffT resolution.
func ComputePTG(simPTG PTG, alpha, dist, diffT float64) ([]*TrajNode, error) {
	if dist < 0 {
		return computeInvertedPTG(simPTG, alpha, dist, diffT)
	}

	// Initialize trajectory with an all-zero node
	alphaTraj := []*TrajNode{{Pose: spatialmath.NewZeroPose()}}

	var err error
	var t, v, w float64
	distTravelled := 0.

	// Step through each time point for this alpha
	for math.Abs(distTravelled) < math.Abs(dist) {
		t += diffT
		nextNode, err := computePTGNode(simPTG, alpha, distTravelled, t)
		if err != nil {
			return nil, err
		}
		v = nextNode.LinVelMMPS
		w = nextNode.AngVelRPS

		// Update velocities of last node because the computed velocities at this node are what should be set after passing the last node.
		// Reasoning: if the distance passed in is 0, then we want the first node to return velocity 0. However, if we want a nonzero
		// distance such that we return two nodes, then the first node, which has zero translation, should set a nonzero velocity so that
		// the next node, which has a nonzero translation, is arrived at when it ought to be.
		alphaTraj[len(alphaTraj)-1].LinVelMMPS = v
		alphaTraj[len(alphaTraj)-1].AngVelRPS = w

		alphaTraj = append(alphaTraj, nextNode)
		displacement := diffT
		switch {
		case v != 0:
			displacement = math.Copysign(math.Abs(v)*diffT, dist)
		case w != 0:
			displacement = math.Copysign(math.Abs(w)*diffT, dist)
		}
		distTravelled += displacement
	}

	// Add final node
	alphaTraj[len(alphaTraj)-1].LinVelMMPS = v
	alphaTraj[len(alphaTraj)-1].AngVelRPS = w
	pose, err := simPTG.Transform([]referenceframe.Input{{alpha}, {dist}})
	if err != nil {
		return nil, err
	}
	tNode := &TrajNode{pose, t, dist, alpha, v, w}
	alphaTraj = append(alphaTraj, tNode)
	return alphaTraj, nil
}

// computePTGNode will return the TrajNode of the requested PTG, at the specified alpha and dist. The provided time is used
// to fill in the time field.
func computePTGNode(simPTG PTG, alpha, dist, atT float64) (*TrajNode, error) {
	v, w, err := simPTG.Velocities(alpha, dist)
	if err != nil {
		return nil, err
	}

	// ptgIK caches these, so this should be fast. If cacheing is removed or a different simPTG used, this could be slow.
	pose, err := simPTG.Transform([]referenceframe.Input{{alpha}, {dist}})
	if err != nil {
		return nil, err
	}
	return &TrajNode{pose, atT, dist, alpha, v, w}, nil
}

func computeInvertedPTG(simPTG PTG, alpha, dist, diffT float64) ([]*TrajNode, error) {
	forwardsPTG, err := ComputePTG(simPTG, alpha, dist*-1, diffT)
	if err != nil {
		return nil, err
	}
	flippedTraj := make([]*TrajNode, 0, len(forwardsPTG))
	startNode := forwardsPTG[len(forwardsPTG)-1] // Cache for convenience

	for i := len(forwardsPTG) - 1; i >= 0; i-- {
		fwdNode := forwardsPTG[i]
		flippedPose := spatialmath.PoseBetween(
			spatialmath.Compose(startNode.Pose, flipPose), spatialmath.Compose(fwdNode.Pose, flipPose),
		)
		flippedTraj = append(flippedTraj,
			&TrajNode{
				flippedPose,
				startNode.Time - fwdNode.Time,
				startNode.Dist - fwdNode.Dist,
				startNode.Alpha,
				fwdNode.LinVelMMPS,
				fwdNode.AngVelRPS * -1,
			})
	}
	return flippedTraj, nil
}

// PTGSegmentMetric is a metric which returns the TP-space distance traversed in a segment. Since PTG inputs are relative, the distance
// travelled is the distance field of the ending configuration.
func PTGSegmentMetric(segment *ik.Segment) float64 {
	return segment.EndConfiguration[distanceAlongTrajectoryIndex].Value
}
