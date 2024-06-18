package tpspace

import (
	"math"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	turnStraightConst = 1.2 // turn at max for this many radians, then go straight, depending on alpha
)

// ptgCS defines a PTG family combined of two stages; first driving forwards while turning at radius, going straight.
// Alpha determines how far to turn before going straight.
type ptgCS struct {
	circle       *ptgC
	turnStraight float64
}

// NewCSPTG creates a new PTG of type ptgCS.
func NewCSPTG(turnRadius float64) PTG {
	circle := NewCirclePTG(turnRadius).(*ptgC)
	turnStraight := turnStraightConst * turnRadius
	return &ptgCS{
		circle:       circle,
		turnStraight: turnStraight,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgCS) Velocities(alpha, dist float64) (float64, float64, error) {
	if dist == 0 {
		return 0, 0, nil
	}
	if dist < ptg.turnDist(alpha) {
		return ptg.circle.Velocities(alpha, dist)
	}

	return 1., 0, nil
}

func (ptg *ptgCS) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	alpha := inputs[0].Value
	dist := inputs[1].Value

	turnDist := ptg.turnDist(alpha)
	var err error
	arcPose := spatialmath.NewZeroPose()
	if alpha != 0 {
		arcPose, err = ptg.circle.Transform([]referenceframe.Input{inputs[0], {math.Min(dist, turnDist)}})
		if err != nil {
			return nil, err
		}
	}
	if dist < turnDist {
		return arcPose, nil
	}
	fwdPose, err := ptg.circle.Transform([]referenceframe.Input{{0}, {dist - turnDist}})
	if err != nil {
		return nil, err
	}
	return spatialmath.Compose(arcPose, fwdPose), nil
}

// turnDist calculates the arc distance of a turn given an alpha value.
func (ptg *ptgCS) turnDist(alpha float64) float64 {
	// Magic number; rotate this much before going straight
	return math.Sqrt(math.Abs(alpha)) * ptg.turnStraight
}
