package tpspace

import (
	"math"
)

// ptgDiffDriveC defines a PTG family composed of circular trajectories with an alpha-dependent radius.
type ptgDiffDriveC struct {
	maxMps   float64 // meters per second velocity to target
	maxRadps float64 // degrees per second of rotation when driving at maxMps and turning at max turning radius
	k        float64 // k = +1 for forwards, -1 for backwards
}

// NewCirclePTG creates a new PrecomputePTG of type ptgDiffDriveC.
func NewCirclePTG(maxMps, maxRadps, k float64) PrecomputePTG {
	return &ptgDiffDriveC{
		maxMps:   maxMps,
		maxRadps: maxRadps,
		k:        k,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgDiffDriveC) PTGVelocities(alpha, t, x, y, phi float64) (float64, float64, error) {
	// (v,w)
	v := ptg.maxMps * ptg.k * 1000
	// Use a linear mapping:  (Old was: w = tan( alpha/2 ) * W_MAX * sign(K))
	w := (alpha / math.Pi) * ptg.maxRadps * ptg.k
	return v, w, nil
}
