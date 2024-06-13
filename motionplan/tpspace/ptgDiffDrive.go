package tpspace

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

const angleAdjust = 0.99 // ajdust alpha radian conversion by this much to prevent paired 180-degree flips

// ptgDiffDrive defines a PTG family composed of a rotation in place, whose magnitude is determined by alpha, followed by moving straight.
// This is essentially the same as the CS PTG, but with a turning radius of zero.
type ptgDiffDrive struct{}

// NewDiffDrivePTG creates a new PTG of type ptgDiffDrive.
func NewDiffDrivePTG(turnRadius float64) PTG {
	return &ptgDiffDrive{}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
func (ptg *ptgDiffDrive) Velocities(alpha, dist float64) (float64, float64, error) {
	// (v,w)
	if dist == 0 {
		return 0, 0, nil
	}
	if dist <= math.Abs(rdkutils.RadToDeg(alpha))*angleAdjust {
		return 0, math.Copysign(1.0, alpha), nil
	}
	return 1.0, 0, nil
}

// Transform will return the pose for the given inputs. The first input is [-pi, pi]. This corresponds to the direction and amount of
// rotation. For distance, dist is equal to the number of radians rotated plus the number of millimeters of straight motion.
func (ptg *ptgDiffDrive) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	if len(inputs) != 2 {
		return nil, referenceframe.NewIncorrectInputLengthError(len(inputs), 2)
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
	turnAngle := math.Copysign(math.Min(dist, math.Abs(rdkutils.RadToDeg(alpha))), alpha) * angleAdjust

	pose := spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: turnAngle})

	if dist <= math.Abs(rdkutils.RadToDeg(alpha)) {
		return pose, nil
	}

	pt := r3.Vector{0, dist - math.Abs(rdkutils.RadToDeg(alpha)), 0} // Straight line, +Y is "forwards"
	return spatialmath.Compose(pose, spatialmath.NewPoseFromPoint(pt)), nil
}

// curvature of an arc of radius r = 1/r
func (ptg *ptgDiffDrive) Curvature(alpha, dist float64) (float64, error) {
	if dist != 0 {
		turnAngle_deg := math.Copysign(math.Min(dist, math.Abs(rdkutils.RadToDeg(alpha))), alpha) * angleAdjust
		turnAngle_rad := turnAngle_deg * (math.Pi / 180.)
		return turnAngle_rad / dist, nil
	} else {
		return 0, nil
	}

}
