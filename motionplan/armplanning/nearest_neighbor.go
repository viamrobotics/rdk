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

func nearestNeighbor(seed *node, tree rrtMap, nodeDistanceFunc NodeDistanceMetric) *node {
	bestDist := math.Inf(1)
	var best *node
	for k := range tree {
		dist := nodeDistanceFunc(seed, k)
		if dist < bestDist {
			bestDist = dist
			best = k
		}
	}
	return best
}
