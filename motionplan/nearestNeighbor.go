package motionplan

import (
	"context"
	"math"
	"sort"
	"sync"

	"go.viam.com/utils"
)

const defaultNeighborsBeforeParallelization = 1000

type neighborManager struct {
	nnKeys            chan node
	neighbors         chan *neighbor
	nnLock            sync.RWMutex
	seedPos           node
	ready             bool
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
		dist := planOpts.DistanceFunc(&Segment{
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
	tree rrtMap,
) node {
	nm.ready = false
	nm.seedPos = seed

	nm.neighbors = make(chan *neighbor, nm.nCPU)
	nm.nnKeys = make(chan node, len(tree))
	defer close(nm.nnKeys)
	defer close(nm.neighbors)

	for i := 0; i < nm.nCPU; i++ {
		utils.PanicCapturingGo(func() {
			nm.nnWorker(ctx, planOpts)
		})
	}

	for k := range tree {
		nm.nnKeys <- k
	}
	nm.nnLock.Lock()
	nm.ready = true
	nm.nnLock.Unlock()
	var best node
	bestDist := math.Inf(1)
	returned := 0
	for returned < nm.nCPU {
		select {
		case nn := <-nm.neighbors:
			returned++
			if nn != nil {
				// nn will be nil if the ctx is cancelled
				if nn.dist < bestDist {
					bestDist = nn.dist
					best = nn.node
				}
			}
		default:
		}
	}
	return best
}

func (nm *neighborManager) nnWorker(ctx context.Context, planOpts *plannerOptions) {
	var best node
	bestDist := math.Inf(1)

	for {
		select {
		case <-ctx.Done():
			nm.neighbors <- nil
			return
		default:
		}

		select {
		case k := <-nm.nnKeys:
			if k != nil {
				nm.nnLock.RLock()
				seg := &Segment{
					StartConfiguration: nm.seedPos.Q(),
					EndConfiguration:   k.Q(),
				}
				if pose := nm.seedPos.Pose(); pose != nil {
					seg.StartPosition = pose
				}
				if pose := k.Pose(); pose != nil {
					seg.EndPosition = pose
				}
				dist := planOpts.DistanceFunc(seg)
				nm.nnLock.RUnlock()
				if dist < bestDist {
					bestDist = dist
					best = k
				}
			}
		default:
			nm.nnLock.RLock()
			if nm.ready {
				nm.nnLock.RUnlock()
				nm.neighbors <- &neighbor{bestDist, best}
				return
			}
			nm.nnLock.RUnlock()
		}
	}
}
