package tpspace

import (
	"math"
)

// ptgDiffDriveCC defines a PTG family combined of two stages; first reversing while turning at radius, then moving forwards while turning
// at radius, resulting in a path that looks like a "3"
// Alpha determines how far to reverse before moving forwards.
type ptgDiffDriveCC struct {
	maxMps   float64 // meters per second velocity to target
	maxRadps float64 // degrees per second of rotation when driving at maxMps and turning at max turning radius
	k        float64 // k = +1 for forwards, -1 for backwards
}

// NewCCPTG creates a new PrecomputePTG of type ptgDiffDriveCC.
func NewCCPTG(maxMps, maxRadps, k float64) PrecomputePTG {
	return &ptgDiffDriveCC{
		maxMps:   maxMps,
		maxRadps: maxRadps,
		k:        k,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgDiffDriveCC) PTGVelocities(alpha, t, x, y, phi float64) (float64, float64, error) {
	r := ptg.maxMps / ptg.maxRadps

	u := math.Abs(alpha) * 0.5

	v := 0.
	w := 0.

	if t < u*r/ptg.maxMps {
		// l-
		v = -ptg.maxMps
		w = ptg.maxRadps
	} else if t < (u+math.Pi*0.5)*r/ptg.maxMps {
		// l+
		v = ptg.maxMps
		w = ptg.maxRadps
	}

	// Turn in the opposite direction??
	if alpha < 0 {
		w *= -1
	}

	v *= ptg.k
	w *= ptg.k
	// m to mm
	v *= 1000

	return v, w, nil
}
