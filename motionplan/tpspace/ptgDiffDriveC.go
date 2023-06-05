package tpspace

import(
	"math"
	
	rutils "go.viam.com/rdk/utils"
)

// This does something with circles
// Other ptgs will be based on this ptg somehow
type ptgDiffDriveC struct {
	resolution float64 // mm
	refDist float64
	numPaths uint
	maxMps float64
	maxDps float64
	turnRad float64 // mm
	k      float64 // k = +1 for forwards, -1 for backwards
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
func (ptg *ptgDiffDriveC) ptgDiffDriveSteer(alpha, t, x, y, phi float64) (float64, float64, error) {
	// (v,w)
	v := ptg.maxMps * ptg.k
	// Use a linear mapping:  (Old was: w = tan( alpha/2 ) * W_MAX * sign(K))
	w := (alpha / math.Pi) * ptg.maxDps * ptg.k
	return v, w, nil
}

// x and y are relative?
// map xy cartesian to d&k
func (ptg *ptgDiffDriveC) WorldSpaceToTP(x, y float64) (uint, float64, error) {
	var d float64
	var k uint
	if y != 0 {
		var theta float64
		r := (x*x + y*y)/(y*2)
		rMin := ptg.maxMps/rutils.DegToRad(ptg.maxDps)
		
		if ptg.k >= 0 {
			if y > 0 {
				theta = math.Atan2(x, math.Abs(r) - y)
			} else {
				theta = math.Atan2(x, math.Abs(r) + y)
			}
		} else {
			if y > 0 {
				theta = math.Atan2(-x, math.Abs(r) - y)
			} else {
				theta = math.Atan2(-x, math.Abs(r) + y)
			}
		}
		
		theta = wrapTo2Pi(theta)
		
		// Distance through arc
		d = theta * (math.Abs(r))
		if math.Abs(r) < rMin {
			r = math.Copysign(rMin, r)
		}
		
		a := math.Pi * ptg.maxMps/(r * rutils.DegToRad(ptg.maxDps))
		k = alpha2index(a, ptg.numPaths)
		
	} else {
		if math.Signbit(x) == math.Signbit(ptg.k) {
			k = alpha2index(0, ptg.numPaths)
			d = x
		} else {
			k = ptg.numPaths - 1
			d = 1e+3
		}
	}
	
	// normalize
	d = d / ptg.refDist
	
	return k, d, nil
}
