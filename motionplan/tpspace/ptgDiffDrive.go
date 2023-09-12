package tpspace

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// ptgDiffDrive defines a PTG family composed of a rotation in place, whose magnitude is determined by alpha, followed by moving straight.
// This is essentially the same as the CS PTG, but with a turning radius of zero.
type ptgDiffDrive struct {
	maxMMPS float64 // millimeters per second velocity to target
	maxRPS  float64 // radians per second of rotation when driving at maxMMPS and turning at max turning radius
}

// NewDiffDrivePTG creates a new PTG of type ptgDiffDrive.
func NewDiffDrivePTG(maxMMPS, maxRPS float64) PTG {
	return &ptgDiffDrive{
		maxMMPS: maxMMPS,
		maxRPS:  maxRPS,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
func (ptg *ptgDiffDrive) Velocities(alpha, dist float64) (float64, float64, error) {
	// (v,w)
	if dist == 0 {
		return 0, 0, nil
	}
	k := math.Copysign(1.0, dist)
	if dist <= math.Abs(alpha) {
		return 0, math.Copysign(ptg.maxRPS, alpha), nil
	}
	return ptg.maxMMPS * k, 0, nil
}

// Transform will return the pose for the given inputs. The first input is [-pi, pi]. This corresponds to the direction and amount of
// of rotation. For distance, dist is equal to the number of radians rotated plus the number of millimeters of straight motion.
func (ptg *ptgDiffDrive) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	if len(inputs) != 2 {
		return nil, referenceframe.NewIncorrectInputLengthError(len(inputs), 2)
		// ~ return nil, fmt.Errorf("ptgDiffDrive takes 2 inputs, but received %d", len(inputs))
	}
	alpha := inputs[0].Value
	dist := inputs[1].Value

	// Check for OOB within FP error
	if math.Pi-math.Abs(alpha) > math.Pi+floatEpsilon {
		return nil, fmt.Errorf("ptgDiffDrive input 0 is limited to [-pi, pi] but received %f", inputs[0])
	}

	if alpha > math.Pi {
		alpha = math.Pi
	}
	if alpha < -1*math.Pi {
		alpha = -1 * math.Pi
	}
	turnAngle := math.Copysign(math.Min(dist, math.Abs(alpha)), alpha)

	pose := spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVector{OZ: 1, Theta: turnAngle})

	if dist <= math.Abs(alpha) {
		return pose, nil
	}

	pt := r3.Vector{0, dist - math.Abs(alpha), 0} // Straight line, +Y is "forwards"
	return spatialmath.Compose(pose, spatialmath.NewPoseFromPoint(pt)), nil
}
