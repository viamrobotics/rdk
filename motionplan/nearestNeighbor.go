package motionplan

import (
	"context"
	"math"
	"sort"

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

//nolint:revive
func kNearestNeighbors(planOpts *plannerOptions, rrtMap map[node]node, target node, neighborhoodSize int) []*neighbor {
	kNeighbors := neighborhoodSize
	if neighborhoodSize > len(rrtMap) {
		kNeighbors = len(rrtMap)
	}

	allCosts := make([]*neighbor, 0)
	for rrtnode := range rrtMap {
		dist := planOpts.DistanceFunc(&Segment{
			StartConfiguration: target.Q(),
			EndConfiguration:   rrtnode.Q(),
		})
		allCosts = append(allCosts, &neighbor{dist: dist, node: rrtnode})
	}
	sort.Slice(allCosts, func(i, j int) bool {
		if !math.IsNaN(allCosts[i].node.Cost()) {
			if !math.IsNaN(allCosts[j].node.Cost()) {
				return (allCosts[i].dist + allCosts[i].node.Cost()) < (allCosts[j].dist + allCosts[j].node.Cost())
			}
		}
		return allCosts[i].dist < allCosts[j].dist
	})
	return allCosts[:kNeighbors]
}

// Can return `nil` when the context is canceled during processing.
func (nm *neighborManager) nearestNeighbor(
	ctx context.Context,
	planOpts *plannerOptions,
	seed node,
	rrtMap map[node]node,
) node {
	if nm.parallelNeighbors == 0 {
		nm.parallelNeighbors = defaultNeighborsBeforeParallelization
	}

	if len(rrtMap) > nm.parallelNeighbors && nm.nCPU > 1 {
		// If the map is large, calculate distances in parallel
		return nm.parallelNearestNeighbor(ctx, planOpts, seed, rrtMap)
	}
	bestDist := math.Inf(1)
	var best node
	for k := range rrtMap {
		seg := &Segment{
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
	rrtMap map[node]node,
) node {
	nm.seedPos = seed

	nm.neighbors = make(chan *neighbor, nm.nCPU)
	nm.nnKeys = make(chan node, len(rrtMap))
	defer close(nm.neighbors)

	for i := 0; i < nm.nCPU; i++ {
		utils.PanicCapturingGo(func() {
			nm.nnWorker(ctx, planOpts)
		})
	}

	for k := range rrtMap {
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

		seg := &Segment{
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
