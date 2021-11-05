package motionplan

import (
	"context"
	"fmt"
	"sort"

	//~ "runtime"
	"testing"

	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

var (
	
	interp = frame.FloatsToInputs([]float64{0.22034293025523666, 0.023301860367034785, 0.0035938741832804775, 0.03706780636626979, -0.006010542176591475, 0.013764993693680328, 0.22994099248696265})
)

// This should test a simple linear motion
func TestExtend(t *testing.T) {
	nSolutions := 5
	inputSteps := [][]frame.Input{}
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	m, err := kinematics.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm7_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)
	
	mp, err := NewCBiRRTMotionPlanner_petertest (m, logger, 4)
	test.That(t, err, test.ShouldBeNil)
	
	pos := &pb.ArmPosition{
		X:  206,
		Y:  100,
		Z:  120,
		OZ: -1,
	}
	
	solutions, err := getSolutions(ctx, mp.frame, mp.solver, pos, home7, mp.constraintHandler)
	test.That(t, err, test.ShouldBeNil)
	
	near1 := &solution{home7}
	seedMap := make(map[*solution]*solution)
	seedMap[near1] = nil
	target := &solution{interp}

	
	
	keys := make([]float64, 0, len(solutions))
	for k := range solutions {
		keys = append(keys, k)
	}
	sort.Float64s(keys)

	goalMap := make(map[*solution]*solution)

	if len(keys) < nSolutions {
		nSolutions = len(keys)
	}

	for _, k := range keys[:nSolutions] {
		fmt.Println("goal", k, solutions[k])
		goalMap[&solution{solutions[k]}] = nil
	}

	seedReached, goalReached := mp.constrainedExtendWrapper(seedMap, goalMap, near1, target)
	
	//~ fmt.Println(target)
	//~ fmt.Println("seedR", seedReached)
	//~ fmt.Println("goalR", goalReached)
	//~ fmt.Println("seedMap")
	//~ printMap(seedMap)
	//~ fmt.Println("goalMap")
	//~ printMap(goalMap)
	
	if inputDist(seedReached.inputs, goalReached.inputs) < mp.solDist {
		fmt.Println("got path!")
		// extract the path to the seed
		for seedReached != nil {
			inputSteps = append(inputSteps, seedReached.inputs)
			seedReached = seedMap[seedReached]
		}
		//~ fmt.Println("path1", inputSteps)
		// reverse the slice
		for i, j := 0, len(inputSteps)-1; i < j; i, j = i+1, j-1 {
			inputSteps[i], inputSteps[j] = inputSteps[j], inputSteps[i]
		}
		// extract the path to the goal
		for goalReached != nil {
			inputSteps = append(inputSteps, goalReached.inputs)
			goalReached = goalMap[goalReached]
		}
		//~ fmt.Println("path", inputSteps)
		inputSteps = mp.smoothPath(ctx, inputSteps)
		//~ fmt.Println(inputSteps)
	}
}

func printMap(m map[*solution]*solution){
	for k, v := range m {
		fmt.Println(k)
		fmt.Println("  -> ", v)
	}
}
