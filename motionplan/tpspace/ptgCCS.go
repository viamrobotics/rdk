package tpspace

import (
	"math"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// ptgCCS defines a PTG family combining the CC and CS trajectories, essentially executing the CC trajectory
// followed by a straight line.
type ptgCCS struct {
	turnRadius float64
	circle     *ptgC
}

// NewCCSPTG creates a new PTG of type ptgCCS.
func NewCCSPTG(turnRadius float64) PTG {
	circle := NewCirclePTG(turnRadius).(*ptgC)

	return &ptgCCS{
		turnRadius: turnRadius,
		circle:     circle,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgCCS) Velocities(alpha, dist float64) (float64, float64, error) {
	if dist == 0 {
		return 0, 0, nil
	}
	u := math.Abs(alpha) * 0.5

	v := 1.0
	w := 0.

	if dist < u*ptg.turnRadius {
		// backwards arc
		v = -1.
		w = 1.
	} else if dist < (u+math.Pi/2)*ptg.turnRadius {
		// forwards arc
		v = 1.
		w = 1.
	}

	// Turn in the opposite direction
	if alpha < 0 {
		w *= -1
	}

	return v, w, nil
}

func (ptg *ptgCCS) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	alpha := inputs[0].Value
	dist := inputs[1].Value

	arcConstant := math.Abs(alpha) * 0.5
	reverseDistance := arcConstant * ptg.turnRadius
	fwdArcDistance := (arcConstant + math.Pi/2) * ptg.turnRadius
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

// curvature of an arc of radius r = 1/r
func (ptg *ptgCCS) Curvature(alpha, dist float64) (float64, error) {
	arcConstant := math.Abs(alpha) * 0.5
	reverseDistance := arcConstant * ptg.turnRadius
	fwdArcDistance := (arcConstant + math.Pi/2) * ptg.turnRadius

	// First C
	curvRev, _ := ptg.circle.Curvature(math.Pi, math.Min(dist, reverseDistance))
	curvRev = math.Abs(curvRev)

	// Second CS
	totalLengthCS := dist - math.Min(dist, reverseDistance)
	if totalLengthCS == 0 {
		return curvRev, nil //no CS part because its equal to 0
	}
	if alpha != 0 {
		arcRadius := math.Pi * ptg.turnRadius / math.Abs(alpha) // radius of arc
		angleRads := fwdArcDistance / arcRadius                 // angle of arc
		return angleRads / totalLengthCS, nil
	} else {
		return 0, nil // Is this correct? Return 0 if alpha == 0
	}

	// curvForw, _ := ptg.circle.Curvature(math.Pi, math.Min(dist, fwdArcDistance)-reverseDistance)
	// curvForw = math.Abs(curvForw)
	// return curvRev + curvForw, nil
}
