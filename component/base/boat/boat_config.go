package boat

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	"go.uber.org/multierr"
	"gonum.org/v1/gonum/mat"
	
	"github.com/go-nlopt/nlopt"
)



type boatConfig struct {
	Motors        []motorConfig
	Length, Width float64
	IMU string
	
	allPossibilites [][]float64
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

func (bc *boatConfig) weights() []motorWeights {
	res := make([]motorWeights, len(bc.Motors))
	for idx, mc := range bc.Motors {
		w := mc.computeWeights(math.Hypot(bc.Width, bc.Length))
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


func (bc *boatConfig) help(m int, cur []float64) {
	if m >= len(bc.Motors) {
		bc.allPossibilites = append(bc.allPossibilites, cur)
		return
	}
	
	for p := -1.0; p <= 1; p += .25 {
		t := make([]float64, len(bc.Motors))
		copy(t, cur)
		t[m] = p
		bc.help(m+1, t)
	}
}

func (bc *boatConfig) computeAllPosibilites() [][]float64 {

	if len(bc.allPossibilites) == 0 {
		bc.help(0, make([]float64, len(bc.Motors)))
	}
	return bc.allPossibilites
}

func sumPower(x []float64) float64 {
	t := 0.0
	for _, p := range x {
		t += math.Abs(p)
	}
	return t
}

// returns an array of power for each motors
// forwardPercent: -1 -> 1 percent of power in which you want to move laterally
//                  note only x & y are relevant. y is forward back, x is lateral
// angularPercent: -1 -> 1 percent of power you want applied to move angularly
//                 note only z is relevant here
func (bc *boatConfig) computePower(linear, angular r3.Vector) []float64 {
	//fmt.Printf("linear: %v angular: %v\n", linear, angular)

	goal := bc.computeGoal(linear, angular)
	
	allPossibilites := bc.computeAllPosibilites()

	powers := []float64{}
	bestDiff := 10000.0
	for _, p := range allPossibilites {

		diff := goal.diff(bc.computePowerOutput(p))
		if diff < bestDiff && math.Abs(diff - bestDiff) > .1 {
			bestDiff = diff
			powers = p
		} else if diff - .02 < bestDiff && sumPower(p) < sumPower(powers) {
			bestDiff = diff
			powers = p
		}
	}
	if false {
		fmt.Printf("goal-pre %v\n", bc.computeGoal(linear, angular))
		fmt.Printf("res-pre  %v\n", bc.computePowerOutput(powers))

		opt, err := nlopt.NewNLopt(nlopt.LD_MMA, 6)
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

			opt.SetFtolAbs(.01),
			opt.SetFtolRel(.01),
			opt.SetStopVal(.01),
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

			total := bc.computePowerOutput(x)
			diff := total.diff(goal)
			fmt.Printf("diff %v\n", diff)
			
			for idx, _ := range(gradient) {
				gradient[idx] = math.Max(.05, diff)
			}

			return diff
		}

		opt.SetMinObjective(myfunc)
		powers, _, err = opt.Optimize(powers)
		if err != nil {
			panic(err)
		}
	}

	//fmt.Printf("\tpowers: %v\n", powers)
	//fmt.Printf("\tgoal-post %v\n", bc.computeGoal(linear, angular))
	//fmt.Printf("\tres-post  %v\n", bc.computePowerOutput(powers))
	
	return powers

}
