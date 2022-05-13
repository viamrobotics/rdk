package boat

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"github.com/golang/geo/r3"

	"github.com/go-nlopt/nlopt"
	
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
		angular: -1 * percentDistanceFromCenterOfMass * mc.Weight * math.Sin(utils.DegToRad(angleOffset)),
	}
}

type boatConfig struct {
	Motors        []motorConfig
	Length, Width float64
}

func (bc *boatConfig) maxWeights() motorWeights {
	var max motorWeights
	for _, mc := range bc.Motors {
		w := mc.computeWeights(math.Hypot(bc.Width, bc.Length))
		max.linearX += math.Abs(w.linearX)
		max.linearY += math.Abs(w.linearY)
		max.angular += math.Abs(w.angular)
	}
	return max
}

func (bc *boatConfig) computeGoal(linear, angular r3.Vector) motorWeights {
	w := bc.maxWeights()
	w.linearX *= linear.X
	w.linearY *= linear.Y
	w.angular *= angular.Z
	return w
}

func (bc *boatConfig) applyMotors(powers []float64) motorWeights {
	if len(powers) != len(bc.Motors) {
		panic(fmt.Errorf("different number of powers (%d) to motors (%d)", len(powers), len(bc.Motors)))
	}
	total := motorWeights{}

	for idx, mc := range bc.Motors {
		w := mc.computeWeights(math.Hypot(bc.Width, bc.Length))
		total.linearX += w.linearX * powers[idx]
		total.linearY += w.linearY * powers[idx]
		total.angular += w.angular * powers[idx]
	}
	
	return total
}

func powerLowLevel(desire, weight float64) float64 {
	if math.Abs(desire) < .05 || math.Abs(weight) < 0.05 {
		return 0
	}

	p := desire

	if weight < 0 {
		p *= -1
	}
	
	return p
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

		p += powerLowLevel(linear.X, w.linearX)
		p += powerLowLevel(linear.Y, w.linearY)
		p += powerLowLevel(angular.Z, w.angular)

		fmt.Printf("\t w: %#v power: %v\n", w, p)
		powers = append(powers, p)
	}

	fmt.Printf("powers: %v\n", powers)
	
	if false {
		goal := bc.computeGoal(linear, angular)
		
		opt, err := nlopt.NewNLopt(nlopt.LD_MMA, 2)
		if err != nil {
			panic(err)
		}
		defer opt.Destroy()
		
		mins := []float64{}
		maxs := []float64{}
		
		for _, _ = range bc.Motors {
			mins = append(mins, -1)
			maxs = append(maxs, -1)
		}
		
		err = multierr.Combine(
			opt.SetLowerBounds(mins),
			opt.SetUpperBounds(maxs),
			opt.SetXtolAbs1(.01),
			opt.SetXtolRel(.01),
		)
		if err != nil {
			panic(1)
		}
		
		var evals int
		myfunc := func(x, gradient []float64) float64 {
			fmt.Printf("yo: %v\n", x)
			evals++
			
			total := bc.applyMotors(x)
			diff := math.Pow(total.linearX - goal.linearX,2) +
				math.Pow(total.linearY - goal.linearY,2) +
				math.Pow(total.angular - goal.angular,2)
			diff = math.Sqrt(diff)
			
			if len(gradient) > 0 {
				
			}
			
			return diff
		}
		
		
		opt.SetMinObjective(myfunc)
		powers, _, err = opt.Optimize(powers)
		if err != nil {
			panic(err)
		}
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
	power := b.cfg.computePower(linear, angular)

	ctx, done := b.opMgr.New(ctx)
	defer done()

	for idx, p := range power {
		err := b.motors[idx].SetPower(ctx, p)
		if err != nil {
			return multierr.Combine(b.Stop(ctx), err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
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
