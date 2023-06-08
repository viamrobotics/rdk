package tpspace

import(
	"math"
	
	rutils "go.viam.com/rdk/utils"
)

// This does something with circles
// Other ptgs will be based on this ptg somehow
type ptgDiffDriveCS struct {
	resolution float64 // mm
	refDist float64
	numPaths uint
	maxMps float64
	maxDps float64
	turnRad float64 // mm
	k      float64 // k = +1 for forwards, -1 for backwards
}

func NewCSPTG(maxMps, maxDps, k float64) PrecomputePTG {
	return &ptgDiffDriveCS{
		maxMps: maxMps,
		maxDps: maxDps,
		k: k,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgDiffDriveCS) PtgDiffDriveSteer(alpha, t, x, y, phi float64) (float64, float64, error) {
	
	r := ptg.maxMps / rutils.DegToRad(ptg.maxDps)
	
	// Magic number; rotate this much before going straight
	// More = more rotation
	//~ turnStraight := 0.847 * math.Sqrt(math.Abs(alpha)) * r / ptg.maxMps
	turnStraight := 1.2 * math.Sqrt(math.Abs(alpha)) * r / ptg.maxMps

	v := ptg.maxMps
	w := 0.

	if (t < turnStraight){
		// l+
		v = ptg.maxMps
		w = rutils.DegToRad(ptg.maxDps) * math.Min(1.0, 1.0 - math.Exp(-1 * alpha * alpha))
		//~ w = rutils.DegToRad(ptg.maxDps) * 1.0 - math.Exp(-1 * alpha * alpha)
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

