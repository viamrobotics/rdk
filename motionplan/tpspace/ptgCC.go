package tpspace

import (
	"math"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// ptgCC defines a PTG family combined of two stages; first reversing while turning at radius, then moving forwards while turning
// at radius, resulting in a path that looks like a "3"
// Alpha determines how far to reverse before moving forwards.
type ptgCC struct {
	turnRadius float64
	circle     *ptgC
}

// NewCCPTG creates a new PTG of type ptgCC.
func NewCCPTG(turnRadius float64) PTG {
	circle := NewCirclePTG(turnRadius).(*ptgC)

	return &ptgCC{
		turnRadius: turnRadius,
		circle:     circle,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgCC) Velocities(alpha, dist float64) (float64, float64, error) {
	if dist == 0 {
		return 0, 0, nil
	}
	u := math.Abs(alpha) * 0.5

	v := 1.0
	w := 1.0

	if dist < u*ptg.turnRadius {
		// This is the reverse part of the trajectory
		v = -1.0
		w = 1.0
	}

	// Turn in the opposite direction
	if alpha < 0 {
		w *= -1
	}

	return v, w, nil
}

func (ptg *ptgCC) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	alpha := inputs[0].Value
	dist := inputs[1].Value
	reverseDistance := math.Abs(alpha) * 0.5 * ptg.turnRadius
	flip := math.Copysign(1., alpha)     // left or right
	direction := math.Copysign(1., dist) // forwards or backwards

	revPose, err := ptg.circle.Transform([]referenceframe.Input{{-1 * flip * math.Pi}, {-1. * direction * math.Min(dist, reverseDistance)}})
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

func (ptg *ptgCC) Curvature(alpha, dist float64) (float64, error) {
	reverseDistance := math.Abs(alpha) * 0.5 * ptg.turnRadius
	curvRev, _ := ptg.circle.Curvature(math.Pi, math.Min(dist, reverseDistance))
	curvRev = math.Abs(curvRev)
	curvForw, _ := ptg.circle.Curvature(math.Pi, dist-reverseDistance)
	curvForw = math.Abs(curvForw)
	return curvRev + curvForw, nil
	// arcRadius := 2 * math.Pi * ptg.turnRadius / math.Abs(alpha) // radius of arc
	// return 1 / arcRadius, nil
}
