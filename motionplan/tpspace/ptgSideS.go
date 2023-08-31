package tpspace

import (
	"math"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const defaultCountersteer = 1.5

// ptgSideS defines a PTG family which makes a forwards turn, then a counter turn the other direction, and goes straight.
// This has the effect of translating to one side or the other without orientation change.
type ptgSideS struct {
	maxMMPS      float64 // millimeters per second velocity to target
	maxRPS       float64 // radians per second of rotation when driving at maxMMPS and turning at max turning radius
	r            float64 // turning radius
	countersteer float64 // scale the length of the second arc by this much
	circle       *ptgC
}

// NewSideSPTG creates a new PTG of type ptgSideS.
func NewSideSPTG(maxMMPS, maxRPS float64) PTG {
	r := maxMMPS / maxRPS
	circle := NewCirclePTG(maxMMPS, maxRPS).(*ptgC)

	return &ptgSideS{
		maxMMPS:      maxMMPS,
		maxRPS:       maxRPS,
		r:            r,
		countersteer: 1.0,
		circle:       circle,
	}
}

// NewSideSOverturnPTG creates a new PTG of type ptgSideS which overturns.
// It turns X amount in one direction, then countersteers X*countersteerFactor in the other direction.
func NewSideSOverturnPTG(maxMMPS, maxRPS float64) PTG {
	r := maxMMPS / maxRPS
	circle := NewCirclePTG(maxMMPS, maxRPS).(*ptgC)

	return &ptgSideS{
		maxMMPS:      maxMMPS,
		maxRPS:       maxRPS,
		r:            r,
		countersteer: defaultCountersteer,
		circle:       circle,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgSideS) PTGVelocities(alpha, dist float64) (float64, float64, error) {
	arcLength := math.Abs(alpha) * 0.5 * ptg.r
	v := ptg.maxMMPS
	w := 0.
	flip := math.Copysign(1., alpha) // left or right

	if dist < arcLength {
		// l-
		v = ptg.maxMMPS
		w = ptg.maxRPS * flip
	} else if dist < arcLength+arcLength*ptg.countersteer {
		v = ptg.maxMMPS
		w = ptg.maxRPS * -1 * flip
	}

	return v, w, nil
}

func (ptg *ptgSideS) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	alpha := inputs[0].Value
	dist := inputs[1].Value

	flip := math.Copysign(1., alpha)           // left or right
	direction := math.Copysign(1., dist)       // forwards or backwards
	arcLength := math.Abs(alpha) * 0.5 * ptg.r //

	revPose, err := ptg.circle.Transform([]referenceframe.Input{{flip * math.Pi}, {direction * math.Min(dist, arcLength)}})
	if err != nil {
		return nil, err
	}
	if dist < arcLength {
		return revPose, nil
	}
	fwdPose, err := ptg.circle.Transform(
		[]referenceframe.Input{
			{-1 * flip * math.Pi},
			{direction * (math.Min(dist, arcLength+arcLength*ptg.countersteer) - arcLength)},
		},
	)
	if err != nil {
		return nil, err
	}
	arcPose := spatialmath.Compose(revPose, fwdPose)
	if dist < arcLength+arcLength*ptg.countersteer {
		return arcPose, nil
	}

	finalPose, err := ptg.circle.Transform([]referenceframe.Input{{0}, {direction * (dist - (arcLength + arcLength*ptg.countersteer))}})
	if err != nil {
		return nil, err
	}
	return spatialmath.Compose(arcPose, finalPose), nil
}
