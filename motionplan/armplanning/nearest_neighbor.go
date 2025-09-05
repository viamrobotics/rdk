//go:build !no_cgo

package armplanning

import (
	"math"
)

func nearestNeighbor(seed node, tree rrtMap, nodeDistanceFunc NodeDistanceMetric) node {
	bestDist := math.Inf(1)
	var best node
	for k := range tree {
		dist := nodeDistanceFunc(seed, k)
		if dist < bestDist {
			bestDist = dist
			best = k
		}
	}
	return best
}
