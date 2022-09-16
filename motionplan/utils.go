package motionplan

import (
	"math"

	commonpb "go.viam.com/api/common/v1"
)

// fixOvIncrement will detect whether the given goal position is a precise orientation increment of the current
// position, in which case it will detect whether we are leaving a pole. If we are an OV increment and leaving a pole,
// then Theta will be adjusted to give an expected smooth movement. The adjusted goal will be returned. Otherwise the
// original goal is returned.
// Rationale: if clicking the increment buttons in the interface, the user likely wants the most intuitive motion
// posible. If setting values manually, the user likely wants exactly what they requested.
func fixOvIncrement(pos, seed *commonpb.Pose) *commonpb.Pose {
	epsilon := 0.0001
	// Nothing to do for spatial translations or theta increments
	if pos.X != seed.X || pos.Y != seed.Y || pos.Z != seed.Z || pos.Theta != seed.Theta {
		return pos
	}
	// Check if seed is pointing directly at pole
	if 1-math.Abs(seed.OZ) > epsilon || pos.OZ != seed.OZ {
		return pos
	}

	// we only care about negative xInc
	xInc := pos.OX - seed.OX
	yInc := math.Abs(pos.OY - seed.OY)
	var adj float64
	if pos.OX == seed.OX {
		// no OX movement
		if yInc != 0.1 && yInc != 0.01 {
			// nonstandard increment
			return pos
		}
		// If wanting to point towards +Y and OZ<0, add 90 to theta, otherwise subtract 90
		if pos.OY-seed.OY > 0 {
			adj = 90
		} else {
			adj = -90
		}
	} else {
		if (xInc != -0.1 && xInc != -0.01) || pos.OY != seed.OY {
			return pos
		}
		// If wanting to point towards -X, increment by 180. Values over 180 or under -180 will be automatically wrapped
		adj = 180
	}
	if pos.OZ > 0 {
		adj *= -1
	}

	return &commonpb.Pose{
		X:     pos.X,
		Y:     pos.Y,
		Z:     pos.Z,
		Theta: pos.Theta + adj,
		OX:    pos.OX,
		OY:    pos.OY,
		OZ:    pos.OZ,
	}
}
