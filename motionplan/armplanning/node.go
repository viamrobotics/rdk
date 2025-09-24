package armplanning

import (
	"context"
	"fmt"
	"math"

	"go.viam.com/rdk/referenceframe"
)

// fixedStepInterpolation returns inputs at qstep distance along the path from start to target.
func fixedStepInterpolation(start, target *node, qstep map[string][]float64) referenceframe.FrameSystemInputs {
	newNear := make(referenceframe.FrameSystemInputs)

	for frameName, startInputs := range start.inputs {
		// As this is constructed in-algorithm from already-near nodes, this is guaranteed to always exist
		targetInputs := target.inputs[frameName]
		frameSteps := make([]referenceframe.Input, len(startInputs))

		qframe, ok := qstep[frameName]
		for j, nearInput := range startInputs {
			v1, v2 := nearInput.Value, targetInputs[j].Value

			step := 0.0
			if ok {
				step = qframe[j]
			}
			if step > math.Abs(v2-v1) {
				frameSteps[j] = referenceframe.Input{Value: v2}
			} else if v1 < v2 {
				frameSteps[j] = referenceframe.Input{Value: nearInput.Value + step}
			} else {
				frameSteps[j] = referenceframe.Input{Value: nearInput.Value - step}
			}
		}
		newNear[frameName] = frameSteps
	}
	return newNear
}

type node struct {
	inputs    referenceframe.FrameSystemInputs
	corner    bool
	cost      float64
	checkPath bool
}

func newConfigurationNode(q referenceframe.FrameSystemInputs) *node {
	return &node{
		inputs: q,
		corner: false,
	}
}

// nodePair groups together nodes in a tuple
// TODO(rb): in the future we might think about making this into a list of nodes.
type nodePair struct{ a, b *node }

func extractPath(startMap, goalMap rrtMap, pair *nodePair, matched bool) []*node {
	// need to figure out which of the two nodes is in the start map
	var startReached, goalReached *node
	if _, ok := startMap[pair.a]; ok {
		startReached, goalReached = pair.a, pair.b
	} else {
		startReached, goalReached = pair.b, pair.a
	}

	// extract the path to the seed
	path := make([]*node, 0)
	for startReached != nil {
		path = append(path, startReached)
		startReached = startMap[startReached]
	}

	// reverse the slice
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	if goalReached != nil {
		if matched {
			// skip goalReached node and go directly to its parent in order to not repeat this node
			goalReached = goalMap[goalReached]
		}

		// extract the path to the goal
		for goalReached != nil {
			path = append(path, goalReached)
			goalReached = goalMap[goalReached]
		}
	}
	return path
}

// This function is the entrypoint to IK for all cases. Everything prior to here is poses or configurations as the user passed in, which
// here are converted to a list of nodes which are to be used as the from or to states for a motionPlanner.
func generateNodeListForPlanState(
	ctx context.Context,
	mp *cBiRRTMotionPlanner,
	state *PlanState,
	ikSeed referenceframe.FrameSystemInputs,
) ([]*node, error) {
	nodes := []*node{}

	if len(state.configuration) > 0 {
		nodes = append(nodes, newConfigurationNode(state.configuration))
		return nodes, nil
	}

	if len(state.poses) != 0 {
		solutions, err := mp.getSolutions(ctx, ikSeed, state.poses)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, solutions...)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("could not create any nodes for state %v", state)
	}
	return nodes, nil
}
