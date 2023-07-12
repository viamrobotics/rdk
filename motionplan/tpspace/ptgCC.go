package tpspace

import (
	"math"
)

// ptgDiffDriveCC defines a PTG family combined of two stages; first reversing while turning at radius, then moving forwards while turning
// at radius, resulting in a path that looks like a "3"
// Alpha determines how far to reverse before moving forwards.
type ptgDiffDriveCC struct {
	maxMMPS float64 // millimeters per second velocity to target
	maxRPS  float64 // radians per second of rotation when driving at maxMMPS and turning at max turning radius
	k       float64 // k = +1 for forwards, -1 for backwards
}

// NewCCPTG creates a new PrecomputePTG of type ptgDiffDriveCC.
func NewCCPTG(maxMMPS, maxRPS, k float64) PrecomputePTG {
	return &ptgDiffDriveCC{
		maxMMPS: maxMMPS,
		maxRPS:  maxRPS,
		k:       k,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgDiffDriveCC) PTGVelocities(alpha, t, x, y, phi float64) (float64, float64, error) {
	r := ptg.maxMMPS / ptg.maxRPS

	u := math.Abs(alpha) * 0.5

	v := 0.
	w := 0.

	if t < u*r/ptg.maxMMPS {
		// l-
		v = -ptg.maxMMPS
		w = ptg.maxRPS
	} else if t < (u+math.Pi*0.5)*r/ptg.maxMMPS {
		// l+
		v = ptg.maxMMPS
		w = ptg.maxRPS
	}

	// Turn in the opposite direction??
	if alpha < 0 {
		w *= -1
	}

	v *= ptg.k
	w *= ptg.k

	return v, w, nil
}
