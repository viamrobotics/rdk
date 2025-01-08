//go:build !no_cgo

package motionplan

import (
	"context"
	"fmt"
	"math"

	"go.viam.com/rdk/referenceframe"
)

// fixedStepInterpolation returns inputs at qstep distance along the path from start to target
// if start and target have the same Input value, then no step increment is made.
func fixedStepInterpolation(start, target node, qstep map[string][]float64) referenceframe.FrameSystemInputs {
	newNear := make(referenceframe.FrameSystemInputs)

	// Iterate through each frame's inputs
	for frameName, startInputs := range start.Q() {
		// As this is constructed in-algorithm from already-near nodes, this is guaranteed to always exist
		targetInputs := target.Q()[frameName]
		frameSteps := make([]referenceframe.Input, len(startInputs))

		for j, nearInput := range startInputs {
			if nearInput.Value == targetInputs[j].Value {
				frameSteps[j] = nearInput
			} else {
				v1, v2 := nearInput.Value, targetInputs[j].Value
				newVal := math.Min(qstep[frameName][j], math.Abs(v2-v1))
				// get correct sign
				newVal *= (v2 - v1) / math.Abs(v2-v1)
				frameSteps[j] = referenceframe.Input{Value: nearInput.Value + newVal}
			}
		}
		newNear[frameName] = frameSteps
	}
	return newNear
}

// node interface is used to wrap a configuration for planning purposes.
// TODO: This is somewhat redundant with a State.
type node interface {
	// return the configuration associated with the node
	Q() referenceframe.FrameSystemInputs
	Cost() float64
	SetCost(float64)
	Poses() referenceframe.FrameSystemPoses
	Corner() bool
	SetCorner(bool)
}

type basicNode struct {
	q      referenceframe.FrameSystemInputs
	cost   float64
	poses  referenceframe.FrameSystemPoses
	corner bool
}

// Special case constructors for nodes without costs to return NaN.
func newConfigurationNode(q referenceframe.FrameSystemInputs) node {
	return &basicNode{
		q:      q,
		cost:   math.NaN(),
		corner: false,
	}
}

func (n *basicNode) Q() referenceframe.FrameSystemInputs {
	return n.q
}

func (n *basicNode) Cost() float64 {
	return n.cost
}

func (n *basicNode) SetCost(cost float64) {
	n.cost = cost
}

func (n *basicNode) Poses() referenceframe.FrameSystemPoses {
	return n.poses
}

func (n *basicNode) Corner() bool {
	return n.corner
}

func (n *basicNode) SetCorner(corner bool) {
	n.corner = corner
}

// nodePair groups together nodes in a tuple
// TODO(rb): in the future we might think about making this into a list of nodes.
type nodePair struct{ a, b node }

func (np *nodePair) sumCosts() float64 {
	aCost := np.a.Cost()
	if math.IsNaN(aCost) {
		return 0
	}
	bCost := np.b.Cost()
	if math.IsNaN(bCost) {
		return 0
	}
	return aCost + bCost
}

func extractPath(startMap, goalMap map[node]node, pair *nodePair, matched bool) []node {
	// need to figure out which of the two nodes is in the start map
	var startReached, goalReached node
	if _, ok := startMap[pair.a]; ok {
		startReached, goalReached = pair.a, pair.b
	} else {
		startReached, goalReached = pair.b, pair.a
	}

	// extract the path to the seed
	path := make([]node, 0)
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

func sumCosts(path []node) float64 {
	cost := 0.
	for _, wp := range path {
		cost += wp.Cost()
	}
	return cost
}

// This function is the entrypoint to IK for all cases. Everything prior to here is poses or configurations as the user passed in, which
// here are converted to a list of nodes which are to be used as the from or to states for a motionPlanner.
func generateNodeListForPlanState(
	ctx context.Context,
	mp motionPlanner,
	state *PlanState,
	ikSeed referenceframe.FrameSystemInputs,
) ([]node, error) {
	nodes := []node{}
	if len(state.poses) != 0 {
		// If we have goal state poses, add them to the goal state configurations
		goalMetric := mp.opt().getGoalMetric(state.poses)
		// get many potential end goals from IK solver
		solutions, err := mp.getSolutions(ctx, ikSeed, goalMetric)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, solutions...)
	}
	if len(state.configuration) > 0 {
		nodes = append(nodes, newConfigurationNode(state.configuration))
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("could not create any nodes for state %v", state)
	}
	return nodes, nil
}
