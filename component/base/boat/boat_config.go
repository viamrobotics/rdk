package boat

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	"go.uber.org/multierr"

	"github.com/go-nlopt/nlopt"
)

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

		for range bc.Motors {
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
			diff := math.Pow(total.linearX-goal.linearX, 2) +
				math.Pow(total.linearY-goal.linearY, 2) +
				math.Pow(total.angular-goal.angular, 2)
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
