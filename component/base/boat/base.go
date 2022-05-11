package boat

import (

	//"fmt".
	"math"


	"github.com/golang/geo/r3"

	"go.viam.com/rdk/utils"
)

type motorWeights struct {
	linearX float64
	linearY float64
	angular float64
}

type motorConfig struct {
	Name           string
	LateralOffset  float64
	VerticalOffset float64
	AngleDegrees   float64 // 0 is thrusting forward, 90 is thrusting to starboard, or positive x
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

	percentDistanceFromCenterOfMass :=
		math.Sqrt(mc.LateralOffset*mc.LateralOffset+mc.VerticalOffset*mc.VerticalOffset) /
			radius

	angleOffset := mc.AngleDegrees - angleFromCenter

	// fmt.Printf("angle: %v angleFromCenter: %v angleOffset: %v percentDistanceFromCenterOfMass: %0.2f, x: %0.2f y: %0.2f\n", mc.AngleDegrees, angleFromCenter, angleOffset, percentDistanceFromCenterOfMass, x, y)

	return motorWeights{
		linearX: x,
		linearY: y,
		angular: percentDistanceFromCenterOfMass * mc.Weight * math.Sin(utils.DegToRad(angleOffset)),
	}
}

type boatConfig struct {
	Motors        []motorConfig
	Length, Width float64
}

// returns an array of power for each motors
// forwardPercent: -1 -> 1 percent of power in which you want to move laterally
//                  note only x & y are relevant. y is forward back, x is lateral
// angularPercent: -1 -> 1 percent of power you want applied to move angularly
//                 note only z is relevant here
func (bc *boatConfig) computePower(linear, angular r3.Vector) []float64 {
	panic(1)
}
