package tpspace

import (
	"math"
)

// Pi / 4 (45 degrees), used as a default alpha constant
// This controls how tightly our parabolas arc
// 57 degrees is also sometimes used by the reference.
const quarterPi = 0.78539816339

// simPtgAlpha defines a PTG family which follows a parabolic path.
type simPTGAlpha struct {
	maxMps   float64 // meters per second velocity to target
	maxRadps float64 // degrees per second of rotation when driving at maxMps and turning at max turning radius
}

// NewAlphaPTG creates a new PrecomputePTG of type simPtgAlpha.
// K is unused for alpha PTGs *for now* but we may add in the future.
func NewAlphaPTG(maxMps, maxRadps, k float64) PrecomputePTG {
	return &simPTGAlpha{
		maxMps:   maxMps,
		maxRadps: maxRadps,
	}
}

func (ptg *simPTGAlpha) PTGVelocities(alpha, t, x, y, phi float64) (float64, float64, error) {
	// In order to know what to set our angvel at, we need to know how far into the path we are
	atA := wrapTo2Pi(alpha - phi)
	if atA > math.Pi {
		atA -= 2 * math.Pi
	}

	v := ptg.maxMps * math.Exp(-1.*math.Pow(atA/quarterPi, 2)) * 1000
	w := ptg.maxRadps * (-0.5 + (1. / (1. + math.Exp(-atA/quarterPi))))

	return v, w, nil
}
