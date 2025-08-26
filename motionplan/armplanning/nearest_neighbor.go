//go:build !no_cgo

package armplanning

import (
	"context"
	"math"

	"go.viam.com/utils"
)

const defaultNeighborsBeforeParallelization = 1000

type neighborManager struct {
	nnKeys            chan node
	neighbors         chan *neighbor
	seedPos           node
	nCPU              int
	parallelNeighbors int
}

type neighbor struct {
	dist float64
	node node
}

// Can return `nil` when the context is canceled during processing.
func (nm *neighborManager) nearestNeighbor(
	ctx context.Context,
	seed node,
	tree rrtMap,
	nodeDistanceFunc NodeDistanceMetric,
) node {
	if nm.parallelNeighbors == 0 {
		nm.parallelNeighbors = defaultNeighborsBeforeParallelization
	}

	if len(tree) > nm.parallelNeighbors && nm.nCPU > 1 {
		// If the map is large, calculate distances in parallel
		return nm.parallelNearestNeighbor(ctx, seed, tree, nodeDistanceFunc)
	}
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

func (nm *neighborManager) parallelNearestNeighbor(
	ctx context.Context,
	seed node,
	tree rrtMap,
	nodeDistanceFunc NodeDistanceMetric,
) node {
	nm.seedPos = seed

	nm.neighbors = make(chan *neighbor, nm.nCPU)
	nm.nnKeys = make(chan node, len(tree))
	defer close(nm.neighbors)

	for i := 0; i < nm.nCPU; i++ {
		utils.PanicCapturingGo(func() {
			nm.nnWorker(ctx, nodeDistanceFunc)
		})
	}

	for k := range tree {
		nm.nnKeys <- k
	}
	close(nm.nnKeys)

	wasInterrupted := false
	var best node
	bestDist := math.Inf(1)
	for workerIdx := 0; workerIdx < nm.nCPU; workerIdx++ {
		candidate := <-nm.neighbors
		if candidate == nil {
			// Seeing a `nil` here implies the workers did not get to all of the candidate
			// neighbors. And thus we don't have the right answer to return.
			wasInterrupted = true
			continue
		}

		if candidate.dist < bestDist {
			bestDist = candidate.dist
			best = candidate.node
		}
	}
	if wasInterrupted {
		return nil
	}

	return best
}

func (nm *neighborManager) nnWorker(ctx context.Context, nodeDistanceFunc NodeDistanceMetric) {
	var best node
	bestDist := math.Inf(1)

	for candidate := range nm.nnKeys {
		select {
		case <-ctx.Done():
			// We were interrupted, signal that to the caller by returning a `nil`.
			nm.neighbors <- nil
			return
		default:
		}

		dist := nodeDistanceFunc(nm.seedPos, candidate)
		if dist < bestDist {
			bestDist = dist
			best = candidate
		}
	}

	nm.neighbors <- &neighbor{bestDist, best}
}
