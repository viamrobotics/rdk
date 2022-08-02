package motionplan

import (
	"context"
	"math"
	"sync"

	"go.viam.com/utils"
)

type neighborManager struct {
	nnKeys    chan *configuration
	neighbors chan *neighbor
	nnLock    sync.RWMutex
	seedPos   *configuration
	ready     bool
	nCPU      int
}

type neighbor struct {
	dist float64
	q    *configuration
}

func (nm *neighborManager) nearestNeighbor(
	ctx context.Context,
	seed *configuration,
	rrtMap map[*configuration]*configuration,
) *configuration {
	if len(rrtMap) > 1000 {
		// If the map is large, calculate distances in parallel
		return nm.parallelNearestNeighbor(ctx, seed, rrtMap)
	}
	bestDist := math.Inf(1)
	var best *configuration
	for k := range rrtMap {
		dist := inputDist(seed.inputs, k.inputs)
		if dist < bestDist {
			bestDist = dist
			best = k
		}
	}
	return best
}

func (nm *neighborManager) parallelNearestNeighbor(
	ctx context.Context,
	seed *configuration,
	rrtMap map[*configuration]*configuration,
) *configuration {
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
	var best *configuration
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
				best = nn.q
			}
		default:
		}
	}
	return best
}

func (nm *neighborManager) startNNworkers(ctx context.Context) {
	nm.neighbors = make(chan *neighbor, nm.nCPU)
	nm.nnKeys = make(chan *configuration, nm.nCPU)
	for i := 0; i < nm.nCPU; i++ {
		utils.PanicCapturingGo(func() {
			nm.nnWorker(ctx)
		})
	}
}

func (nm *neighborManager) nnWorker(ctx context.Context) {
	var best *configuration
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
				dist := inputDist(nm.seedPos.inputs, k.inputs)
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
