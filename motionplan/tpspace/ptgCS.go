package tpspace

import (
	"math"

	rutils "go.viam.com/rdk/utils"
)

// ptgDiffDriveCS defines a PTG family combined of two stages; first driving forwards while turning at radius, going straight.
// Alpha determines how far to turn before going straight.
type ptgDiffDriveCS struct {
	maxMps float64 // meters per second velocity to target
	maxDps float64 // degrees per second of rotation when driving at maxMps and turning at max turning radius
	k      float64 // k = +1 for forwards, -1 for backwards
}

// NewCSPTG creates a new PrecomputePTG of type ptgDiffDriveCS.
func NewCSPTG(maxMps, maxDps, k float64) PrecomputePTG {
	return &ptgDiffDriveCS{
		maxMps: maxMps,
		maxDps: maxDps,
		k:      k,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgDiffDriveCS) PTGVelocities(alpha, t, x, y, phi float64) (float64, float64, error) {
	r := ptg.maxMps / rutils.DegToRad(ptg.maxDps)

	// Magic number; rotate this much before going straight
	// Bigger value = more rotation
	turnStraight := 1.2 * math.Sqrt(math.Abs(alpha)) * r / ptg.maxMps

	v := ptg.maxMps
	w := 0.

	if t < turnStraight {
		// l+
		v = ptg.maxMps
		w = rutils.DegToRad(ptg.maxDps) * math.Min(1.0, 1.0-math.Exp(-1*alpha*alpha))
	}

	// Turn in the opposite direction
	if alpha < 0 {
		w *= -1
	}

	v *= ptg.k
	w *= ptg.k
	// m to mm
	v *= 1000
	return v, w, nil
}
