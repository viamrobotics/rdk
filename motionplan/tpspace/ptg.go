//go:build !windows

// Package tpspace defines an assortment of precomputable trajectories which can be used to plan nonholonomic 2d motion
package tpspace

import (
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/spatialmath"
)

// PTG is a Parameterized Trajectory Generator, which defines how to map back and forth from cartesian space to TP (alpha, d)
// One of these is needed for each sort of motion that can be done
// Forwards PTG converts TP-space to C-space
// Inverse PTG converts C-space to TP-space.
type PTG interface {
	// WorldSpaceToTP Converts an x, y world space coord to a k, d (alpha index plus distance) TP-space coord
	// Also returns a bool representing whether the xy is a within-traj match vs an extrapolation
	WorldSpaceToTP(x, y float64) []*TrajNode

	// RefDistance returns the maximum distance that a single precomputed trajectory may travel
	RefDistance() float64

	// Returns the set of trajectory nodes for alpha index K
	Trajectory(uint) []*TrajNode
}

// PTGProvider is something able to provide a set of PTGs associsated with it. For example, a frame which precomputes
// a number of PTGs.
type PTGProvider interface {
	// PTGs returns the list of PTGs associated with this provider
	PTGs() []PTG
}

// PrecomputePTG is a precomputable PTG.
type PrecomputePTG interface {
	// PtgVelocities returns the linear and angular velocity at a specific point along a trajectory
	PtgVelocities(alpha, t, x, y, phi float64) (float64, float64, error)
}

// TrajNode is a snapshot of a single point in time along a PTG trajectory, including the distance along that trajectory,
// the elapsed time along the trajectory, and the linear and angular velocity at that point.
type TrajNode struct {
	// TODO: cache pose point and orientation so that we don't recompute every time we need it
	Pose spatialmath.Pose // for 2d, we only use x, y, and OV theta
	Time float64          // elapsed time on trajectory
	Dist float64          // distance travelled down trajectory
	K    uint             // alpha k-value at this node
	V    float64          // linvel at this node
	W    float64          // angvel at this node

	ptX float64
	ptY float64
}

// discretized path to alpha.
// The inverse of this, which may be useful, looks like this:
// alpha = wrapTo2Pi(alpha)
// k := int(math.Round(0.5 * (float64(numPaths)*(1.0+alpha/math.Pi) - 1.0))).
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

func xyphiToPose(x, y, phi float64) spatialmath.Pose {
	return spatialmath.NewPose(r3.Vector{x, y, 0}, &spatialmath.OrientationVector{OZ: 1, Theta: phi})
}
