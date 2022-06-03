package boat

import (
	"fmt"
	"math"

	"github.com/go-nlopt/nlopt"
	"github.com/golang/geo/r3"
	"go.uber.org/multierr"
	"gonum.org/v1/gonum/mat"
)

type boatConfig struct {
	Motors   []motorConfig
	LengthMM float64 `json:"length_mm"`
	WidthMM  float64 `json:"width_mm"`
	IMU      string
}

func (bc *boatConfig) maxWeights() motorWeights {
	var max motorWeights
	for _, mc := range bc.Motors {
		w := mc.computeWeights(math.Hypot(bc.WidthMM, bc.LengthMM))
		max.linearX += math.Abs(w.linearX)
		max.linearY += math.Abs(w.linearY)
		max.angular += math.Abs(w.angular)
	}
	return max
}

// examples:
//    currentVal=2 otherVal=1, currentGoal=1, otherGoal=1 = 1
//    currentVal=-2 otherVal=1, currentGoal=1, otherGoal=1 = -1

func goalScale(currentVal, otherVal, currentGoal, otherGoal float64) float64 {
	// near 0, do nothing
	if math.Abs(currentGoal) < .05 || math.Abs(otherGoal) < .05 {
		return currentVal
	}

	ratioGoal := math.Abs(currentGoal / otherGoal)
	ratioCur := math.Abs(currentVal / otherVal)

	if ratioCur > ratioGoal {
		currentVal = otherVal * ratioGoal
	}

	return currentVal
}

func (bc *boatConfig) computeGoal(linear, angular r3.Vector) motorWeights {
	w := bc.maxWeights()
	w.linearX *= linear.X
	w.linearY *= linear.Y
	w.angular *= angular.Z

	w.linearX = goalScale(w.linearX, w.linearY, linear.X, linear.Y)
	w.linearX = goalScale(w.linearX, w.angular, linear.X, angular.Z)

	w.linearY = goalScale(w.linearY, w.linearX, linear.Y, linear.X)
	w.linearY = goalScale(w.linearY, w.angular, linear.Y, angular.Z)

	w.angular = goalScale(w.angular, w.linearX, angular.Z, linear.X)
	w.angular = goalScale(w.angular, w.linearY, angular.Z, linear.Y)

	return w
}

func (bc *boatConfig) weights() []motorWeights {
	res := make([]motorWeights, len(bc.Motors))
	for idx, mc := range bc.Motors {
		w := mc.computeWeights(math.Hypot(bc.WidthMM, bc.LengthMM))
		res[idx] = w
	}
	return res
}

func (bc *boatConfig) weightsAsMatrix() *mat.Dense {
	m := mat.NewDense(3, len(bc.Motors), nil)

	for idx, w := range bc.weights() {
		m.Set(0, idx, w.linearX)
		m.Set(1, idx, w.linearY)
		m.Set(2, idx, w.angular)
	}

	return m
}

func (bc *boatConfig) computePowerOutputAsMatrix(powers []float64) mat.Dense {
	if len(powers) != len(bc.Motors) {
		panic(fmt.Errorf("powers wrong length got: %d should be: %d", len(powers), len(bc.Motors)))
	}
	var out mat.Dense

	out.Mul(bc.weightsAsMatrix(), mat.NewDense(len(powers), 1, powers))

	return out
}

func (bc *boatConfig) computePowerOutput(powers []float64) motorWeights {
	out := bc.computePowerOutputAsMatrix(powers)

	return motorWeights{
		linearX: out.At(0, 0),
		linearY: out.At(1, 0),
		angular: out.At(2, 0),
	}
}

// returns an array of power for each motors
// forwardPercent: -1 -> 1 percent of power in which you want to move laterally
//                  note only x & y are relevant. y is forward back, x is lateral
// angularPercent: -1 -> 1 percent of power you want applied to move angularly
//                 note only z is relevant here
func (bc *boatConfig) computePower(linear, angular r3.Vector) []float64 {
	goal := bc.computeGoal(linear, angular)

	opt, err := nlopt.NewNLopt(nlopt.GN_DIRECT, 6)
	if err != nil {
		panic(err)
	}
	defer opt.Destroy()

	mins := []float64{}
	maxs := []float64{}

	for range bc.Motors {
		mins = append(mins, -1)
		maxs = append(maxs, 1)
	}

	err = multierr.Combine(
		opt.SetLowerBounds(mins),
		opt.SetUpperBounds(maxs),

		opt.SetStopVal(.002),
		opt.SetMaxTime(.25),
	)
	if err != nil {
		panic(err)
	}

	myfunc := func(x, gradient []float64) float64 {
		total := bc.computePowerOutput(x)
		return total.diff(goal)
	}

	err = opt.SetMinObjective(myfunc)
	if err != nil {
		panic(err)
	}
	powers, _, err := opt.Optimize(make([]float64, 6))
	if err != nil {
		panic(err)
	}

	return powers
}
