package tpspace

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// ptgC defines a PTG family composed of circular trajectories with an alpha-dependent radius.
type ptgC struct {
	maxMMPS float64 // millimeters per second velocity to target
	maxRPS  float64 // radians per second of rotation when driving at maxMMPS and turning at max turning radius
}

// NewCirclePTG creates a new PTG of type ptgC.
func NewCirclePTG(maxMMPS, maxRPS float64) PTG {
	return &ptgC{
		maxMMPS: maxMMPS,
		maxRPS:  maxRPS,
	}
}

// For this particular driver, turns alpha into a linear + angular velocity. Linear is just max * fwd/back.
// Note that this will NOT work as-is for 0-radius turning. Robots capable of turning in place will need to be special-cased
// because they will have zero linear velocity through their turns, not max.
func (ptg *ptgC) Velocities(alpha, dist float64) (float64, float64, error) {
	// (v,w)
	if dist == 0 {
		return 0, 0, nil
	}
	k := math.Copysign(1.0, dist)
	v := ptg.maxMMPS * k
	w := (alpha / math.Pi) * ptg.maxRPS * k
	return v, w, nil
}

// Transform will return the pose for the given inputs. The first input is [-pi, pi]. This corresponds to the radius of the curve,
// where 0 is straight ahead, pi is turning at min turning radius to the right, and a value between 0 and pi represents turning at a radius
// of (input/pi)*minradius. A negative value denotes turning left. The second input is the distance traveled along this arc.
func (ptg *ptgC) Transform(inputs []referenceframe.Input) (spatialmath.Pose, error) {
	turnRad := ptg.maxMMPS / ptg.maxRPS

	if len(inputs) != 2 {
		return nil, fmt.Errorf("ptgC takes 2 inputs, but received %d", len(inputs))
	}
	alpha := inputs[0].Value
	dist := inputs[1].Value

	// Check for OOB within FP error
	if math.Pi-math.Abs(alpha) > math.Pi+floatEpsilon {
		return nil, fmt.Errorf("ptgC input 0 is limited to [-pi, pi] but received %f", inputs[0])
	}

	if alpha > math.Pi {
		alpha = math.Pi
	}
	if alpha < -1*math.Pi {
		alpha = -1 * math.Pi
	}

	pt := r3.Vector{0, dist, 0} // Straight line, +Y is "forwards"
	angleRads := 0.
	if alpha != 0 {
		arcRadius := math.Pi * turnRad / math.Abs(alpha) // radius of arc
		angleRads = dist / arcRadius                     // number of radians to travel along arc
		pt = r3.Vector{arcRadius * (1 - math.Cos(angleRads)), arcRadius * math.Sin(angleRads), 0}
		if alpha > 0 {
			// positive alpha = positive rotation = left turn = negative X
			pt.X *= -1
			angleRads *= -1
		}
	}
	pose := spatialmath.NewPose(pt, &spatialmath.OrientationVector{OZ: 1, Theta: -angleRads})

	return pose, nil
}
