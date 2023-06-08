//go:build !windows

package tpspace

import (
	"math"
	
	"github.com/golang/geo/r3"
	
	"go.viam.com/rdk/spatialmath"
)

// Struct? Interface?
// A Parameterized Trajectory Generator (PTG), defines how to map back and forth from cartesian space to TP (alpha, d) space and vice versa.
// One of these is needed for each sort of motion that can be done
// Forwards PTG converts TP-space to C-space
// Inverse PTG converts C-space to TP-space
type PTG interface {
	
	// WorldSpaceToTP Converts an x, y world space coord to a k, d (alpha index plus distance) TP-space coord
	// Also returns a bool representing whether the xy is a within-traj match vs an extrapolation
	//~ WorldSpaceToTP(float64, float64) (uint, float64, bool)
	WorldSpaceToTP(float64, float64, float64) []*TrajNode
	
	// RefDistance returns the maximum distance that a single precomputed trajectory may travel
	RefDistance() float64
	
	// Returns the set of trajectory nodes for alpha index K
	Trajectory(uint) []*TrajNode
}

type PTGProvider interface {
	PTGs() []PTG
}

type PrecomputePTG interface {
	// TODO: this should operate on alpha, d rather than the prior t,x,y,phi
	PtgDiffDriveSteer(alpha, t, x, y, phi float64) (float64, float64, error)
}

type TrajNode struct {
	// TODO: cache pose point and orientation so that we don't recompute every time we need it
	Pose spatialmath.Pose // for 2d, we only use x, y, and OV theta
	Time float64 // elapsed time on trajectory
	Dist float64 // distance travelled down trajectory
	K uint // alpha k-value at this node
	V float64 // linvel at this node
	W float64 // angvel at this node
	ptX float64
	ptY float64
}

// discretized path back to alpha
func index2alpha(k, numPaths uint) float64 {
	if k >= numPaths {
		return math.NaN()
	}
	if numPaths == 0 {
		return math.NaN()
	}
	return math.Pi * (-1.0 + 2.0 * (float64(k) + 0.5) / float64(numPaths));
}

// alpha to a discretized path
func alpha2index(alpha float64, numPaths uint) uint {
	alpha = wrapTo2Pi(alpha)
	
	k := int(math.Round(0.5 * (float64(numPaths) * (1.0 + alpha / math.Pi) - 1.0)))
	if k < 0 {
		k = 0
	}
	if k >= int(numPaths) {
		k = int(numPaths) - 1
	}
	return uint(k)
}

// Returns a given angle in the [0, 2pi) range
func wrapTo2Pi(theta float64) float64 {
	return theta - 2 * math.Pi * math.Floor(theta / (2 * math.Pi))
}

func xyphiToPose(x, y, phi float64) spatialmath.Pose {
	return spatialmath.NewPose(r3.Vector{x, y, 0}, &spatialmath.OrientationVector{OZ: 1, Theta: phi})
}
