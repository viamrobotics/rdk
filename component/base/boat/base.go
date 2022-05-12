package boat

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"github.com/golang/geo/r3"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/component/generic"
)

func init() {
	boatComp := registry.Component{
		Constructor: func(
			ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger,
		) (interface{}, error) {
			return createBoat(ctx, r, config.ConvertedAttributes.(*boatConfig), logger)
		},
	}
	registry.RegisterComponent(base.Subtype, "boat", boatComp)

	config.RegisterComponentAttributeMapConverter(
		base.SubtypeName,
		"boat",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf boatConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&boatConfig{})
}

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

	fmt.Printf("\t %v angle: %v angleFromCenter: %v angleOffset: %v percentDistanceFromCenterOfMass: %0.2f, x: %0.2f y: %0.2f\n", mc.Name, mc.AngleDegrees, angleFromCenter, angleOffset, percentDistanceFromCenterOfMass, x, y)

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
	fmt.Printf("linear: %v angular: %v\n", linear, angular)
	powers := []float64{}
	for _, mc := range bc.Motors {
		w := mc.computeWeights(math.Hypot(bc.Width, bc.Length))
		p := 0.0
		if linear.Y > 0 && w.linearY > 0 {
			p = math.Max(linear.Y, w.linearY)
		} else if linear.Y < 0 && w.linearY < 0 {
			p = math.Min(linear.Y, w.linearY)
		} else if angular.Z > 0 && w.angular > 0 {
			p = math.Max(angular.Z, w.angular)
		} else if angular.Z < 0 && w.angular < 0 {
			p = math.Min(angular.Z, w.angular)
		}
		fmt.Printf("\t w: %#v power: %v\n", w, p)
		powers = append(powers, p)
	}
	return powers
}

func createBoat(ctx context.Context, r robot.Robot, config *boatConfig, logger golog.Logger) (base.LocalBase, error) {
	if config.Width <= 0 {
		return nil, errors.New("width has to be > 0")
	}

	if config.Length <= 0 {
		return nil, errors.New("length has to be > 0")
	}

	theBoat := &boat{cfg: config}

	for _, mc := range config.Motors {
		m, err := motor.FromRobot(r, mc.Name)
		if err != nil {
			return nil, err
		}
		theBoat.motors = append(theBoat.motors, m)
	}
	
	fmt.Printf("hi %#v\n", theBoat)

	return theBoat, nil
}

type boat struct {
	generic.Unimplemented

	cfg *boatConfig
	motors []motor.Motor

	opMgr operation.SingleOperationManager
}

func (b *boat) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64) error {
	panic(1)
}

func (b *boat) MoveArc(ctx context.Context, distanceMm int, mmPerSec float64, angleDeg float64) error {
	panic(1)
}

func (b *boat) Spin(ctx context.Context, angleDeg float64, degsPerSec float64) error {
	panic(1)
}

func (b *boat) SetPower(ctx context.Context, linear, angular r3.Vector) error {
	b.opMgr.CancelRunning(ctx)
	power := b.cfg.computePower(linear, angular)

	for idx, p := range power {
		err := b.motors[idx].SetPower(ctx, p)
		if err != nil {
			return multierr.Combine(b.Stop(ctx), err)
		}
	}

	return nil
}
	
func (b *boat) Stop(ctx context.Context) error {
	b.opMgr.CancelRunning(ctx)
	var err error
	for _, m := range b.motors {
		err = multierr.Combine(m.Stop(ctx), err)
	}
	return err
}
	
func (b *boat)GetWidth(ctx context.Context) (int, error) {
	return int(b.cfg.Width) * 1000, nil
}
