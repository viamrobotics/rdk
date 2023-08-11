package tpspace

import (
	"math"
	
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

const (
	turnStraightConst = 1.2 // turn at max for this many radians, then go straight, depending on alpha
)

// ptgDiffDriveCS defines a PTG family combined of two stages; first driving forwards while turning at radius, going straight.
// Alpha determines how far to turn before going straight.
type ptgDiffDriveCS struct {
	maxMMPS float64 // millimeters per second velocity to target
	maxRPS  float64 // radians per second of rotation when driving at maxMMPS and turning at max turning radius
	
	r       float64 // turning radius
	turnStraight float64
}

// NewCSPTG creates a new PrecomputePTG of type ptgDiffDriveCS.
func NewCSPTG(maxMMPS, maxRPS float64) PrecomputePTG {
	r := maxMMPS / maxRPS
	turnStraight := 1.2 * r
	return &ptgDiffDriveCS{
		maxMMPS: maxMMPS,
		maxRPS:  maxRPS,
		r: r,
		turnStraight: turnStraight,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgDiffDriveCS) PTGVelocities(alpha, dist float64) (float64, float64, error) {
	// Magic number; rotate this much before going straight
	turnDist := math.Sqrt(math.Abs(alpha)) * ptg.turnStraight
	k := math.Copysign(1.0, dist)

	v := ptg.maxMMPS
	w := 0.

	if dist < turnDist {
		// l+
		v = ptg.maxMMPS
		w = ptg.maxRPS * math.Min(1.0, 1.0-math.Exp(-1*alpha*alpha))
	}

	// Turn in the opposite direction
	if alpha < 0 {
		w *= -1
	}

	v *= k
	w *= k
	return v, w, nil
}

func (ptg *ptgDiffDriveCS) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	alpha := inputs[0].Value
	dist := inputs[1].Value
	
	actualRPS := ptg.maxRPS * math.Min(1.0, 1.0-math.Exp(-1*alpha*alpha))
	circle := NewCirclePTG(ptg.maxMMPS, actualRPS).(*ptgDiffDriveC)
	
	arcDistance := ptg.turnStraight * math.Sqrt(math.Abs(alpha))
	flip := math.Copysign(1., alpha) // left or right
	direction := math.Copysign(1., dist) // forwards or backwards
	var err error
	arcPose := spatialmath.NewZeroPose()
	if alpha != 0 {
		arcPose, err = circle.Transform([]referenceframe.Input{{flip * math.Pi}, {direction * math.Min(dist, arcDistance)}})
		if err != nil {
			return nil, err
		}
	}
	if dist < arcDistance {
		return arcPose, nil
	}
	fwdPose, err := circle.Transform([]referenceframe.Input{{0}, {direction * (dist - arcDistance)}})
	if err != nil {
		return nil, err
	}
	return spatialmath.Compose(arcPose, fwdPose), nil
}
