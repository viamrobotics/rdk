package motionplan

import (
	"context"
	"math"
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
	nm.ready = false
	nm.seedPos = seed

	nm.neighbors = make(chan *neighbor, nm.nCPU)
	nm.nnKeys = make(chan node, nm.nCPU)
	defer close(nm.nnKeys)
	defer close(nm.neighbors)

	for i := 0; i < nm.nCPU; i++ {
		utils.PanicCapturingGo(func() {
			nm.nnWorker(ctx, planOpts)
		})
	}

	for k := range rrtMap {
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
