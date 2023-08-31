package tpspace

import (
	"math"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// ptgCCS defines a PTG family combining the CC and CS trajectories, essentially executing the CC trajectory
// followed by a straight line.
type ptgCCS struct {
	maxMMPS float64 // millimeters per second velocity to target
	maxRPS  float64 // radians per second of rotation when driving at maxMMPS and turning at max turning radius
	r       float64
	circle  *ptgC
}

// NewCCSPTG creates a new PTG of type ptgCCS.
func NewCCSPTG(maxMMPS, maxRPS float64) PTG {
	r := maxMMPS / maxRPS
	circle := NewCirclePTG(maxMMPS, maxRPS).(*ptgC)

	return &ptgCCS{
		maxMMPS: maxMMPS,
		maxRPS:  maxRPS,
		r:       r,
		circle:  circle,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgCCS) PTGVelocities(alpha, dist float64) (float64, float64, error) {
	u := math.Abs(alpha) * 0.5
	k := math.Copysign(1.0, dist)

	v := ptg.maxMMPS
	w := 0.

	if dist < u*ptg.r {
		// l-
		v = -ptg.maxMMPS
		w = ptg.maxRPS
	} else if dist < (u+math.Pi/2)*ptg.r {
		// l+ pi/2
		v = ptg.maxMMPS
		w = ptg.maxRPS
	}

	// Turn in the opposite direction??
	if alpha < 0 {
		w *= -1
	}

	v *= k
	w *= k

	return v, w, nil
}

func (ptg *ptgCCS) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	alpha := inputs[0].Value
	dist := inputs[1].Value

	arcConstant := math.Abs(alpha) * 0.5
	reverseDistance := arcConstant * ptg.r
	fwdArcDistance := (arcConstant + math.Pi/2) * ptg.r
	flip := math.Copysign(1., alpha)     // left or right
	direction := math.Copysign(1., dist) // forwards or backwards

	revPose, err := ptg.circle.Transform([]referenceframe.Input{{-1 * flip * math.Pi}, {-1. * direction * math.Min(dist, reverseDistance)}})
	if err != nil {
		return nil, err
	}
	if dist < reverseDistance {
		return revPose, nil
	}
	fwdPose, err := ptg.circle.Transform(
		[]referenceframe.Input{
			{flip * math.Pi},
			{direction * (math.Min(dist, fwdArcDistance) - reverseDistance)},
		},
	)
	if err != nil {
		return nil, err
	}
	arcPose := spatialmath.Compose(revPose, fwdPose)
	if dist < reverseDistance+fwdArcDistance {
		return arcPose, nil
	}

	finalPose, err := ptg.circle.Transform([]referenceframe.Input{{0}, {direction * (dist - (fwdArcDistance + reverseDistance))}})
	if err != nil {
		return nil, err
	}
	return spatialmath.Compose(arcPose, finalPose), nil
}
