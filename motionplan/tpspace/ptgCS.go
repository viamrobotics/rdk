package tpspace

import (
	"math"
)

// ptgDiffDriveCS defines a PTG family combined of two stages; first driving forwards while turning at radius, going straight.
// Alpha determines how far to turn before going straight.
type ptgDiffDriveCS struct {
	maxMmps  float64 // millimeters per second velocity to target
	maxRadps float64 // radians per second of rotation when driving at maxMmps and turning at max turning radius
	k        float64 // k = +1 for forwards, -1 for backwards
}

// NewCSPTG creates a new PrecomputePTG of type ptgDiffDriveCS.
func NewCSPTG(maxMmps, maxRadps, k float64) PrecomputePTG {
	return &ptgDiffDriveCS{
		maxMmps:  maxMmps,
		maxRadps: maxRadps,
		k:        k,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgDiffDriveCS) PTGVelocities(alpha, t, x, y, phi float64) (float64, float64, error) {
	r := ptg.maxMmps / ptg.maxRadps

	// Magic number; rotate this much before going straight
	// Bigger value = more rotation
	turnStraight := 1.2 * math.Sqrt(math.Abs(alpha)) * r / ptg.maxMmps

	v := ptg.maxMmps
	w := 0.

	if t < turnStraight {
		// l+
		v = ptg.maxMmps
		w = ptg.maxRadps * math.Min(1.0, 1.0-math.Exp(-1*alpha*alpha))
	}

	// Turn in the opposite direction
	if alpha < 0 {
		w *= -1
	}

	v *= ptg.k
	w *= ptg.k
	return v, w, nil
}
