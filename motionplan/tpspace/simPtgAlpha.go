package tpspace

//~ import (
	//~ "math"
	//~ "errors"
	
	//~ "go.viam.com/rdk/referenceframe"
	//~ "go.viam.com/rdk/spatialmath"
//~ )

//~ // Pi / 4 (45 degrees), used as a default alpha constant
//~ // This controls how tightly our parabolas arc
//~ // 57 degrees is also sometimes used by the reference.
//~ const quarterPi = 0.78539816339

//~ // simPtgAlpha defines a PTG family which follows a parabolic path.
//~ type simPTGAlpha struct {
	//~ maxMMPS float64 // millimeters per second velocity to target
	//~ maxRPS  float64 // radians per second of rotation when driving at maxMMPS and turning at max turning radius
//~ }

//~ // NewAlphaPTG creates a new PrecomputePTG of type simPtgAlpha.
//~ // K is unused for alpha PTGs *for now* but we may add in the future.
//~ func NewAlphaPTG(maxMMPS, maxRPS, k float64) PrecomputePTG {
	//~ return &simPTGAlpha{
		//~ maxMMPS: maxMMPS,
		//~ maxRPS:  maxRPS,
	//~ }
//~ }

//~ func (ptg *simPTGAlpha) PTGVelocities(alpha, t, x, y, phi float64) (float64, float64, error) {
	//~ // In order to know what to set our angvel at, we need to know how far into the path we are
	//~ atA := wrapTo2Pi(alpha - phi)
	//~ if atA > math.Pi {
		//~ atA -= 2 * math.Pi
	//~ }

	//~ v := ptg.maxMMPS * math.Exp(-1.*math.Pow(atA/quarterPi, 2))
	//~ w := ptg.maxRPS * (-0.5 + (1. / (1. + math.Exp(-atA/quarterPi))))

	//~ return v, w, nil
//~ }

//~ func (ptg *simPTGAlpha) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	//~ return nil, errors.New("alpha PTG does not support Transform yet")
//~ }
