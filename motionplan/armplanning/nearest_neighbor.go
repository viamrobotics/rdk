package armplanning

import (
	"math"

	"go.viam.com/rdk/motionplan"
)

// NodeDistanceMetric is a function type used to compute nearest neighbors.
type NodeDistanceMetric func(a, b *node) float64

func nodeConfigurationDistanceFunc(node1, node2 *node) float64 {
	return motionplan.FSConfigurationL2Distance(&motionplan.SegmentFS{
		StartConfiguration: node1.inputs,
		EndConfiguration:   node2.inputs,
	})
}

// nearestNeighbor scans the tree for the node closest to seed. The scan runs
// once per extend over trees that grow to thousands of nodes, so it compares
// the linearized-input slices directly (a field read on LinearInputs) instead
// of going through the frame-map distance metric (which was the planner's
// hottest loop).
func nearestNeighbor(seed *node, tree rrtMap, nodeDistanceFunc NodeDistanceMetric) *node {
	sl := seed.inputs.GetLinearizedInputs()
	bestDist := math.Inf(1)
	var best *node
	for k := range tree {
		kl := k.inputs.GetLinearizedInputs()
		if len(kl) != len(sl) {
			// Mixed schemas: fall back to the full metric for this candidate.
			if dist := nodeDistanceFunc(seed, k); dist < bestDist {
				bestDist = dist
				best = k
			}
			continue
		}
		dist := 0.0
		for i, v := range sl {
			d := v - kl[i]
			dist += d * d
			if dist >= bestDist {
				break
			}
		}
		if dist < bestDist {
			bestDist = dist
			best = k
		}
	}
	return best
}
