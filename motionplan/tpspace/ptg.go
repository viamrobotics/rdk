// Package tpspace defines an assortment of precomputable trajectories which can be used to plan nonholonomic 2d motion
package tpspace

import (
	"context"
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const floatEpsilon = 0.0001 // If floats are closer than this consider them equal

// PTG is a Parameterized Trajectory Generator, which defines how to map back and forth from cartesian space to TP-space
// PTG coordinates are specified in polar coordinates (alpha, d)
// One of these is needed for each sort of motion that can be done.
type PTG interface {
	// CToTP Converts a pose to a (k, d) TP-space trajectory, returning the set of trajectory nodes leading to that pose
	CToTP(context.Context, spatialmath.Pose) []*TrajNode

	// RefDistance returns the maximum distance that a single trajectory may travel
	RefDistance() float64

	// Returns the set of trajectory nodes along the given trajectory, out to the requested distance
	Trajectory(alpha, dist float64) []*TrajNode
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
	PTGVelocities(alpha, t, x, y, phi float64) (float64, float64, error)
	Transform([]referenceframe.Input) (spatialmath.Pose, error)
}

// TrajNode is a snapshot of a single point in time along a PTG trajectory, including the distance along that trajectory,
// the elapsed time along the trajectory, and the linear and angular velocity at that point.
type TrajNode struct {
	// TODO: cache pose point and orientation so that we don't recompute every time we need it
	Pose       spatialmath.Pose // for 2d, we only use x, y, and OV theta
	Time       float64          // elapsed time on trajectory
	Dist       float64          // distance travelled down trajectory
	K          uint             // alpha k-value at this node
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
	alpha = wrapTo2Pi(alpha)
	return uint(math.Round(0.5 * (float64(numPaths)*(1.0+alpha/math.Pi) - 1.0)))
}

// Returns a given angle in the [0, 2pi) range.
func wrapTo2Pi(theta float64) float64 {
	return theta - 2*math.Pi*math.Floor(theta/(2*math.Pi))
}

func xythetaToPose(x, y, theta float64) spatialmath.Pose {
	return spatialmath.NewPose(r3.Vector{x, y, 0}, &spatialmath.OrientationVector{OZ: 1, Theta: theta})
}
