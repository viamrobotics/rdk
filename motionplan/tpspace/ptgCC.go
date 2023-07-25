package tpspace

import (
	"math"
	
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// ptgDiffDriveCC defines a PTG family combined of two stages; first reversing while turning at radius, then moving forwards while turning
// at radius, resulting in a path that looks like a "3"
// Alpha determines how far to reverse before moving forwards.
type ptgDiffDriveCC struct {
	maxMMPS float64 // millimeters per second velocity to target
	maxRPS  float64 // radians per second of rotation when driving at maxMMPS and turning at max turning radius
	k       float64 // k = +1 for forwards, -1 for backwards
	
	circle *ptgDiffDriveC
}

// NewCCPTG creates a new PrecomputePTG of type ptgDiffDriveCC.
func NewCCPTG(maxMMPS, maxRPS, k float64) PrecomputePTG {
	
	circle := NewCirclePTG(maxMMPS, maxRPS, k).(*ptgDiffDriveC)
	
	return &ptgDiffDriveCC{
		maxMMPS: maxMMPS,
		maxRPS:  maxRPS,
		k:       k,
		circle: circle,
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

	// Turn in the opposite direction
	if alpha < 0 {
		w *= -1
	}

	v *= ptg.k
	w *= ptg.k

	return v, w, nil
}

func (ptg *ptgDiffDriveCC) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	alpha := inputs[0].Value
	dist := inputs[1].Value
	
	reverseDistance := math.Abs(alpha) * 0.5
	flip := math.Copysign(1., alpha) // left or right
	direction := math.Copysign(1., dist) // forwards or backwards
	
	revPose, err := ptg.circle.Transform([]referenceframe.Input{{flip * math.Pi}, {-1. * direction * math.Min(dist, reverseDistance)}})
	if err != nil {
		return nil, err
	}
	if dist < reverseDistance {
		return revPose, nil
	}
	fwdPose, err := ptg.circle.Transform([]referenceframe.Input{{flip * math.Pi}, {direction * (dist - reverseDistance)}})
	if err != nil {
		return nil, err
	}
	return spatialmath.Compose(revPose, fwdPose), nil
}
