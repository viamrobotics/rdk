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
	turnRadius   float64 // turning radius
	countersteer float64 // scale the length of the second arc by this much
	circle       *ptgC
}

// NewSideSPTG creates a new PTG of type ptgSideS.
func NewSideSPTG(turnRadius float64) PTG {
	circle := NewCirclePTG(turnRadius).(*ptgC)

	return &ptgSideS{
		turnRadius:   turnRadius,
		countersteer: 1.0,
		circle:       circle,
	}
}

// NewSideSOverturnPTG creates a new PTG of type ptgSideS which overturns.
// It turns X amount in one direction, then countersteers X*countersteerFactor in the other direction.
func NewSideSOverturnPTG(turnRadius float64) PTG {
	circle := NewCirclePTG(turnRadius).(*ptgC)

	return &ptgSideS{
		turnRadius:   turnRadius,
		countersteer: defaultCountersteer,
		circle:       circle,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgSideS) Velocities(alpha, dist float64) (float64, float64, error) {
	if dist == 0 {
		return 0, 0, nil
	}
	arcLength := math.Abs(alpha) * 0.5 * ptg.turnRadius
	v := 1.0
	w := 0.
	flip := math.Copysign(1., alpha) // left or right

	if dist < arcLength {
		w = 1.0
	} else if dist < arcLength+arcLength*ptg.countersteer {
		w = 1.0 * -1
	}

	return v, w * flip, nil
}

func (ptg *ptgSideS) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	alpha := inputs[0].Value
	dist := inputs[1].Value

	flip := math.Copysign(1., alpha)     // left or right
	direction := math.Copysign(1., dist) // forwards or backwards
	arcLength := math.Abs(alpha) * 0.5 * ptg.turnRadius

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

// curvature of an arc of radius r = 1/r
func (ptg *ptgSideS) Curvature(alpha float64) (float64, error) {
	if alpha != 0 {
		arcRadius := math.Pi / math.Abs(alpha) // radius of arc
		return 1 / arcRadius, nil
	} else { // straight line, therefore curvature = 0
		return 0, nil
	}
}
