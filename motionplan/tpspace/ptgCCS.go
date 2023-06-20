package tpspace

import (
	"math"

	rutils "go.viam.com/rdk/utils"
)

// ptgDiffDriveCCS defines a PTG family combining the CC and CS trajectories, essentially executing the CC trajectory
// followed by a straight line.
type ptgDiffDriveCCS struct {
	maxMps float64 // meters per second velocity to target
	maxDps float64 // degrees per second of rotation when driving at maxMps and turning at max turning radius
	k      float64 // k = +1 for forwards, -1 for backwards
}

// NewCCSPTG creates a new PrecomputePTG of type ptgDiffDriveCCS.
func NewCCSPTG(maxMps, maxDps, k float64) PrecomputePTG {
	return &ptgDiffDriveCCS{
		maxMps: maxMps,
		maxDps: maxDps,
		k:      k,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgDiffDriveCCS) PTGVelocities(alpha, t, x, y, phi float64) (float64, float64, error) {
	u := math.Abs(alpha) * 0.5

	r := ptg.maxMps / rutils.DegToRad(ptg.maxDps)

	v := ptg.maxMps
	w := 0.

	if t < u*r/ptg.maxMps {
		// l-
		v = -ptg.maxMps
		w = rutils.DegToRad(ptg.maxDps)
	} else if t < (u+math.Pi/2)*r/ptg.maxMps {
		// l+ pi/2
		v = ptg.maxMps
		w = rutils.DegToRad(ptg.maxDps)
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
