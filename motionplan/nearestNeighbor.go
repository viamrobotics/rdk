package motionplan

import (
	"context"
	"math"
	"sort"
	"sync"

	"go.viam.com/utils"
)

const neighborsBeforeParallelization = 1000

type neighborManager struct {
	nnKeys    chan *node
	neighbors chan *neighbor
	nnLock    sync.RWMutex
	seedPos   *node
	ready     bool
	nCPU      int
}

type neighbor struct {
	dist float64
	node *node
}

func kNearestNeighbors(rrtMap map[*node]*node, target *node) []*neighbor {
	kNeighbors := neighborhoodSize
	if neighborhoodSize > len(rrtMap) {
		kNeighbors = len(rrtMap)
	}

	allCosts := make([]*neighbor, 0)
	for node, _ := range rrtMap {
		allCosts = append(allCosts, &neighbor{dist: inputDist(node.q, target.q), node: node})
	}
	sort.Slice(allCosts, func(i, j int) bool {
		return allCosts[i].dist < allCosts[j].dist
	})
	return allCosts[:kNeighbors]
}

func (nm *neighborManager) nearestNeighbor(
	ctx context.Context,
	seed *node,
	rrtMap map[*node]*node,
) *node {
	if len(rrtMap) > neighborsBeforeParallelization {
		// If the map is large, calculate distances in parallel
		return nm.parallelNearestNeighbor(ctx, seed, rrtMap)
	}
	bestDist := math.Inf(1)
	var best *node
	for k := range rrtMap {
		dist := inputDist(seed.q, k.q)
		if dist < bestDist {
			bestDist = dist
			best = k
		}
	}
	return best
}

func (nm *neighborManager) parallelNearestNeighbor(
	ctx context.Context,
	seed *node,
	rrtMap map[*node]*node,
) *node {
	nm.ready = false
	nm.startNNworkers(ctx)
	defer close(nm.nnKeys)
	defer close(nm.neighbors)
	nm.nnLock.Lock()
	nm.seedPos = seed
	nm.nnLock.Unlock()

	for k := range rrtMap {
		nm.nnKeys <- k
	}
	nm.nnLock.Lock()
	nm.ready = true
	nm.nnLock.Unlock()
	var best *node
	bestDist := math.Inf(1)
	returned := 0
	for returned < nm.nCPU {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		select {
		case nn := <-nm.neighbors:
			returned++
			if nn.dist < bestDist {
				bestDist = nn.dist
				best = nn.node
			}
		default:
		}
	}
	return best
}

func (nm *neighborManager) startNNworkers(ctx context.Context) {
	nm.neighbors = make(chan *neighbor, nm.nCPU)
	nm.nnKeys = make(chan *node, nm.nCPU)
	for i := 0; i < nm.nCPU; i++ {
		utils.PanicCapturingGo(func() {
			nm.nnWorker(ctx)
		})
	}
}

func (nm *neighborManager) nnWorker(ctx context.Context) {
	var best *node
	bestDist := math.Inf(1)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case k := <-nm.nnKeys:
			if k != nil {
				nm.nnLock.RLock()
				dist := inputDist(nm.seedPos.q, k.q)
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
