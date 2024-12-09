// Package tpspace defines an assortment of precomputable trajectories which can be used to plan nonholonomic 2d motion
package tpspace

import (
	"context"
	"errors"
	"math"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	floatEpsilon      = 0.0001 // If floats are closer than this consider them equal
	defaultPTGSeedAdj = 0.2
	defaultResolution = 5.
)

var flipPose = spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 180})

// PTGSolver wraps a PTG with the ability to perform Inverse Kinematics.
type PTGSolver interface {
	referenceframe.Limited
	PTG

	// Returns the set of trajectory nodes along the given trajectory, out to the requested distance.
	// This will return `TrajNode`s starting at dist=start, and every `resolution` increments thereafter, and finally at `end` exactly.
	Trajectory(alpha, start, end, resolution float64) ([]*TrajNode, error)
	// Solve will return the (alpha, dist) TP-space coordinates whose corresponding relative pose minimizes the given function
	Solve(context.Context, []referenceframe.Input, ik.StateMetric) (*ik.Solution, error)
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
	// Velocities returns the linear and angular velocity at a specific point along a trajectory as a [-1, 1] proportion of maximum.
	Velocities(alpha, dist float64) (float64, float64, error)
	Transform([]referenceframe.Input) (spatialmath.Pose, error)
}

// PTGCourseCorrection offers an interface permitting a PTGSolver to also provide an index pointing to one of their PTGSolvers which may
// be used for course correction maneuvers. Usually this is the Circle PTG as it will permit the largest success rate for corrections.
type PTGCourseCorrection interface {
	PTGProvider
	CorrectionSolverIdx() int
}

// TrajNode is a snapshot of a single point in time along a PTG trajectory, including the distance along that trajectory,
// the elapsed time along the trajectory, and the linear and angular velocity at that point.
type TrajNode struct {
	// TODO: cache pose point and orientation so that we don't recompute every time we need it
	Pose   spatialmath.Pose // for 2d, we only use x, y, and OV theta
	Dist   float64          // distance travelled down trajectory
	Alpha  float64          // alpha k-value at this node
	LinVel float64          // Linear velocity as a proportion of maximum at this node [-1, 1]
	AngVel float64          // Angular velocity as a proportion of maximum at this node [-1, 1]
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

// computePTG will compute all nodes of simPTG at the requested alpha, out to the requested distance, at the specified resolution.
func computePTG(simPTG PTG, alpha, dist, resolution float64) ([]*TrajNode, error) {
	if dist < 0 {
		return nil, errors.New("computePTG can only be used with dist values >= zero")
	}
	if resolution <= 0 {
		resolution = defaultResolution
	}

	alphaTraj := []*TrajNode{}
	var err error
	var v, w float64
	distTravelled := 0.
	setFirstVel := false

	// Step through each time point for this alpha
	for math.Abs(distTravelled) < math.Abs(dist) {
		nextNode, err := computePTGNode(simPTG, alpha, distTravelled)
		if err != nil {
			return nil, err
		}
		v = nextNode.LinVel
		w = nextNode.AngVel

		// Update velocities of last node because the computed velocities at this node are what should be set after passing the last node.
		// Reasoning: if the distance passed in is 0, then we want the first node to return velocity 0. However, if we want a nonzero
		// distance such that we return two nodes, then the first node, which has zero translation, should set a nonzero velocity so that
		// the next node, which has a nonzero translation, is arrived at when it ought to be.
		if len(alphaTraj) > 0 && !setFirstVel {
			alphaTraj[0].LinVel = v
			alphaTraj[0].AngVel = w
			setFirstVel = true
		}

		alphaTraj = append(alphaTraj, nextNode)
		distTravelled += resolution
	}
	// Add final node
	lastNode, err := computePTGNode(simPTG, alpha, dist)
	if err != nil {
		return nil, err
	}
	if len(alphaTraj) > 0 {
		alphaTraj[len(alphaTraj)-1].LinVel = lastNode.LinVel
		alphaTraj[len(alphaTraj)-1].AngVel = lastNode.AngVel
	}
	alphaTraj = append(alphaTraj, lastNode)
	return alphaTraj, nil
}

// computePTGNode will return the TrajNode of the requested PTG, at the specified alpha and dist. The provided time is used
// to fill in the time field.
func computePTGNode(simPTG PTG, alpha, dist float64) (*TrajNode, error) {
	v, w, err := simPTG.Velocities(alpha, dist)
	if err != nil {
		return nil, err
	}

	// ptgIK caches these, so this should be fast. If cacheing is removed or a different simPTG used, this could be slow.
	pose, err := simPTG.Transform([]referenceframe.Input{{alpha}, {dist}})
	if err != nil {
		return nil, err
	}
	return &TrajNode{pose, dist, alpha, v, w}, nil
}

func invertComputedPTG(forwardsPTG []*TrajNode) []*TrajNode {
	flippedTraj := make([]*TrajNode, 0, len(forwardsPTG))
	startNode := forwardsPTG[len(forwardsPTG)-1] // Cache for convenience

	for i := len(forwardsPTG) - 1; i >= 0; i-- {
		fwdNode := forwardsPTG[i]
		flippedPose := spatialmath.PoseBetween(
			spatialmath.Compose(startNode.Pose, flipPose), spatialmath.Compose(fwdNode.Pose, flipPose),
		)
		flippedTraj = append(flippedTraj,
			&TrajNode{
				Pose:   flippedPose,
				Dist:   fwdNode.Dist,
				Alpha:  startNode.Alpha,
				LinVel: fwdNode.LinVel,
				AngVel: fwdNode.AngVel * -1,
			})
	}
	return flippedTraj
}

// PTGSegmentMetric is a metric which returns the TP-space distance traversed in a segment. Since PTG inputs are relative, the distance
// travelled is the distance field of the ending configuration.
func PTGSegmentMetric(segment *ik.Segment) float64 {
	return segment.EndConfiguration[len(segment.EndConfiguration)-1].Value
}

// NewPTGDistanceMetric creates a metric which returns the TP-space distance traversed in a segment for a frame. Since PTG inputs are
// relative, the distance travelled is the distance field of the ending configuration.
func NewPTGDistanceMetric(ptgFrames []string) ik.SegmentFSMetric {
	return func(segment *ik.SegmentFS) float64 {
		score := 0.
		for _, ptgFrame := range ptgFrames {
			if frameCfg, ok := segment.EndConfiguration[ptgFrame]; ok {
				score += frameCfg[len(frameCfg)-1].Value
			}
		}
		// If there's no matching configuration in the end, then the frame does not move
		return score
	}
}

// PTGIKSeed will generate a consistent set of valid, in-bounds inputs to be used with a PTGSolver as a seed for gradient descent.
func PTGIKSeed(ptg PTGSolver) []referenceframe.Input {
	inputs := []referenceframe.Input{}
	ptgDof := ptg.DoF()

	// Set the seed to be used for nlopt solving based on the individual DoF range of the PTG.
	// If the DoF only allows short PTGs, seed near the end of its length, otherwise seed near the beginning.
	for i := 0; i < len(ptgDof); i++ {
		boundRange := ptgDof[i].Max - ptgDof[i].Min
		minAdj := boundRange * defaultPTGSeedAdj
		inputs = append(inputs,
			referenceframe.Input{ptgDof[i].Min + minAdj},
		)
	}

	return inputs
}
