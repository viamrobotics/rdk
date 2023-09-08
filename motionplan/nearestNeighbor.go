//go:build !notc

package motionplan

import (
	"context"
	"math"
	"sort"

	"go.viam.com/utils"

	"go.viam.com/rdk/motionplan/ik"
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

//nolint:revive
func kNearestNeighbors(planOpts *plannerOptions, tree rrtMap, target node, neighborhoodSize int) []*neighbor {
	kNeighbors := neighborhoodSize
	if neighborhoodSize > len(tree) {
		kNeighbors = len(tree)
	}

	allCosts := make([]*neighbor, 0)
	for rrtnode := range tree {
		dist := planOpts.DistanceFunc(&ik.Segment{
			StartConfiguration: target.Q(),
			EndConfiguration:   rrtnode.Q(),
		})
		allCosts = append(allCosts, &neighbor{dist: dist, node: rrtnode})
	}
	// sort neighbors by their distance to target first so that first nearest neighbor isn't always the start node of tree
	sort.Slice(allCosts, func(i, j int) bool {
		if !math.IsNaN(allCosts[i].node.Cost()) {
			if !math.IsNaN(allCosts[j].node.Cost()) {
				return allCosts[i].dist < allCosts[j].dist
			}
		}
		return allCosts[i].dist < allCosts[j].dist
	})
	allCosts = allCosts[:kNeighbors]
	// sort k nearest distance neighbors by "total cost to target" metric so that target's nearest neighbor
	// provides the smallest cost path from start node to target
	sort.Slice(allCosts, func(i, j int) bool {
		if !math.IsNaN(allCosts[i].node.Cost()) {
			if !math.IsNaN(allCosts[j].node.Cost()) {
				return (allCosts[i].dist + allCosts[i].node.Cost()) < (allCosts[j].dist + allCosts[j].node.Cost())
			}
		}
		return allCosts[i].dist < allCosts[j].dist
	})
	return allCosts
}

// Can return `nil` when the context is canceled during processing.
func (nm *neighborManager) nearestNeighbor(
	ctx context.Context,
	planOpts *plannerOptions,
	seed node,
	tree rrtMap,
) node {
	if nm.parallelNeighbors == 0 {
		nm.parallelNeighbors = defaultNeighborsBeforeParallelization
	}

	if len(tree) > nm.parallelNeighbors && nm.nCPU > 1 {
		// If the map is large, calculate distances in parallel
		return nm.parallelNearestNeighbor(ctx, planOpts, seed, tree)
	}
	bestDist := math.Inf(1)
	var best node
	for k := range tree {
		seg := &ik.Segment{
			StartConfiguration: seed.Q(),
			EndConfiguration:   k.Q(),
		}
		if pose := seed.Pose(); pose != nil {
			seg.StartPosition = pose
		}
		if pose := k.Pose(); pose != nil {
			seg.EndPosition = pose
		}
		dist := planOpts.DistanceFunc(seg)
		if dist < bestDist {
			bestDist = dist
			best = k
		}
	}
	return best
}

func (nm *neighborManager) parallelNearestNeighbor(
	ctx context.Context,
	planOpts *plannerOptions,
	seed node,
	tree rrtMap,
) node {
	nm.seedPos = seed

	nm.neighbors = make(chan *neighbor, nm.nCPU)
	nm.nnKeys = make(chan node, len(tree))
	defer close(nm.neighbors)

	for i := 0; i < nm.nCPU; i++ {
		utils.PanicCapturingGo(func() {
			nm.nnWorker(ctx, planOpts)
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

func (nm *neighborManager) nnWorker(ctx context.Context, planOpts *plannerOptions) {
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

		seg := &ik.Segment{
			StartConfiguration: nm.seedPos.Q(),
			EndConfiguration:   candidate.Q(),
		}
		if pose := nm.seedPos.Pose(); pose != nil {
			seg.StartPosition = pose
		}
		if pose := candidate.Pose(); pose != nil {
			seg.EndPosition = pose
		}
		dist := planOpts.DistanceFunc(seg)
		if dist < bestDist {
			bestDist = dist
			best = candidate
		}
	}

	nm.neighbors <- &neighbor{bestDist, best}
}
