package boat

import (
	"math"

	"go.viam.com/rdk/utils"
)

type motorWeights struct {
	linearX float64
	linearY float64
	angular float64
}

func (mw *motorWeights) diff(other motorWeights) float64 {
	return math.Sqrt(math.Pow(mw.linearX-other.linearX, 2) +
		math.Pow(mw.linearY-other.linearY, 2) +
		math.Pow(mw.angular-other.angular, 2))
}

type motorConfig struct {
	Name         string
	XOffset      float64 `json:"x_offset"` // in meters
	YOffset      float64 `json:"y_offset"` // in meters
	AngleDegrees float64 `json:"angle"`    // 0 is thrusting forward, 90 is thrusting to starboard, or positive x
	Weight       float64
}

// percentDistanceFromCenterOfMass: if the boat is a circle with a radius of 5m,
// this is the distance from center in m / 5m.
func (mc *motorConfig) computeWeights(radius float64) motorWeights {
	x := math.Sin(utils.DegToRad(mc.AngleDegrees)) * mc.Weight
	y := math.Cos(utils.DegToRad(mc.AngleDegrees)) * mc.Weight

	angleFromCenter := 0.0
	if mc.YOffset == 0 {
		if mc.XOffset > 0 {
			angleFromCenter = 90
		} else if mc.XOffset < 0 {
			angleFromCenter = -90
		}
	} else {
		angleFromCenter = utils.RadToDeg(math.Atan(mc.XOffset / mc.YOffset))
	}

	percentDistanceFromCenterOfMass := math.Hypot(mc.XOffset, mc.YOffset) / radius

	angleOffset := mc.AngleDegrees - angleFromCenter

	return motorWeights{
		linearX: x,
		linearY: y,
		angular: -1 * percentDistanceFromCenterOfMass * mc.Weight * math.Sin(utils.DegToRad(angleOffset)),
	}
}
