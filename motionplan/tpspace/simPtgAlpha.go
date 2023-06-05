package tpspace

import(
	"math"
	
	rutils "go.viam.com/rdk/utils"
)

// Pi / 4 (45 degrees), used as a default alpha constant
const quarterPi = 0.78539816339

// This does something with circles
// Other ptgs will be based on this ptg somehow
type simPtgAlpha struct {
	maxMps float64
	maxDps float64
}

func (ptg *simPtgAlpha) ptgDiffDriveSteer(alpha, t, x, y, phi float64) (float64, float64, error) {
	
	atA := wrapTo2Pi(alpha - phi)
	if atA > math.Pi {
		atA -= 2 * math.Pi
	}
	
	v := ptg.maxMps * math.Exp(-1. * math.Pow(atA/quarterPi, 2))
	w := rutils.DegToRad(ptg.maxDps) * (-0.5 + (1. / (1. + math.Exp(-atA/quarterPi))))
	
	return v, w, nil
}
