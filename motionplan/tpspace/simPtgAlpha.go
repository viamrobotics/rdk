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
	maxMMPS float64 // millimeters per second velocity to target
	maxRPS  float64 // radians per second of rotation when driving at maxMMPS and turning at max turning radius
}

// NewAlphaPTG creates a new PrecomputePTG of type simPtgAlpha.
// K is unused for alpha PTGs *for now* but we may add in the future.
func NewAlphaPTG(maxMMPS, maxRPS, k float64) PrecomputePTG {
	return &simPTGAlpha{
		maxMMPS: maxMMPS,
		maxRPS:  maxRPS,
	}
}

func (ptg *simPTGAlpha) PTGVelocities(alpha, t, x, y, phi float64) (float64, float64, error) {
	// In order to know what to set our angvel at, we need to know how far into the path we are
	atA := wrapTo2Pi(alpha - phi)
	if atA > math.Pi {
		atA -= 2 * math.Pi
	}

	v := ptg.maxMMPS * math.Exp(-1.*math.Pow(atA/quarterPi, 2))
	w := ptg.maxRPS * (-0.5 + (1. / (1. + math.Exp(-atA/quarterPi))))

	return v, w, nil
}

//~ func (ptg *simPTGAlpha) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	//~ alpha := inputs[0].Value
	//~ dist := inputs[1].Value
	
	//~ reverseDistance := math.Abs(alpha) * 0.5
	//~ fwdArcDistance := (reverseDistance + math.Pi/2) * ptg.r
	//~ flip := math.Copysign(1., alpha) // left or right
	//~ direction := math.Copysign(1., dist) // forwards or backwards
	
	//~ revPose, err := ptg.circle.Transform([]referenceframe.Input{{flip * math.Pi}, {-1. * direction * math.Min(dist, reverseDistance)}})
	//~ if err != nil {
		//~ return nil, err
	//~ }
	//~ if dist < reverseDistance {
		//~ return revPose, nil
	//~ }
	//~ fwdPose, err := ptg.circle.Transform([]referenceframe.Input{{flip * math.Pi}, {direction * (math.Min(dist, fwdArcDistance) - reverseDistance)}})
	//~ if err != nil {
		//~ return nil, err
	//~ }
	//~ arcPose := spatialmath.Compose(revPose, fwdPose)
	//~ if dist < reverseDistance + fwdArcDistance {
		//~ return arcPose, nil
	//~ }
	
	//~ finalPose, err := ptg.circle.Transform([]referenceframe.Input{{0}, {direction * (dist - (fwdArcDistance + reverseDistance))}})
	//~ if err != nil {
		//~ return nil, err
	//~ }
	//~ return spatialmath.Compose(arcPose, finalPose), nil
//~ }
