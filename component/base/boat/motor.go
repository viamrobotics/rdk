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

type motorConfig struct {
	Name           string
	LateralOffset  float64 `json:"lateral_offset"`
	VerticalOffset float64 `json:"vertical_offset"`
	AngleDegrees   float64 `json:"angle"` // 0 is thrusting forward, 90 is thrusting to starboard, or positive x
	Weight         float64
}

// percentDistanceFromCenterOfMass: if the boat is a circle with a radius of 5m,
// this is the distance from center in m / 5m.
func (mc *motorConfig) computeWeights(radius float64) motorWeights {
	x := math.Sin(utils.DegToRad(mc.AngleDegrees)) * mc.Weight
	y := math.Cos(utils.DegToRad(mc.AngleDegrees)) * mc.Weight

	angleFromCenter := 0.0
	if mc.VerticalOffset == 0 {
		if mc.LateralOffset > 0 {
			angleFromCenter = 90
		} else if mc.LateralOffset < 0 {
			angleFromCenter = -90
		}
	} else {
		angleFromCenter = utils.RadToDeg(math.Atan(mc.LateralOffset / mc.VerticalOffset))
	}

	percentDistanceFromCenterOfMass := math.Hypot(mc.LateralOffset, mc.VerticalOffset) / radius

	angleOffset := mc.AngleDegrees - angleFromCenter

	return motorWeights{
		linearX: x,
		linearY: y,
		angular: -1 * percentDistanceFromCenterOfMass * mc.Weight * math.Sin(utils.DegToRad(angleOffset)),
	}
}
