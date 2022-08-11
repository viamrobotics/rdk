package rimage

import (
	"image"
	"sync"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/pointcloud"
)

const numThreadsDmPointCloudAdapter = 8 // TODO This should probably become a parameter at some point

func newDMPointCloudAdapter(dm *DepthMap, p Projector) *dmPointCloudAdapter {
	var wg sync.WaitGroup
	wg.Add(2)
	var newDm *DepthMap
	utils.PanicCapturingGo(func() {
		defer wg.Done()
		newDm = dm.Clone()
	})

	var size int
	sizeChan := make(chan int, numThreadsDmPointCloudAdapter)
	utils.PanicCapturingGo(func() {
		defer wg.Done()
		var sizeWg sync.WaitGroup
		sizeWg.Add(numThreadsDmPointCloudAdapter)
		// Round up to avoid missing points
		batchSize := ((dm.width * dm.height) + numThreadsDmPointCloudAdapter - 1) / numThreadsDmPointCloudAdapter
		for loop := 0; loop < numThreadsDmPointCloudAdapter; loop++ {
			f := func(loop int) {
				defer sizeWg.Done()
				sizeBuf := 0
				for i := 0; i < batchSize; i++ {
					x := loop*batchSize + i
					if x >= dm.width*dm.height {
						break
					}
					depth := dm.GetDepth(x%dm.width, x/dm.width)
					if depth == 0 {
						continue
					}
					sizeBuf++
				}
				sizeChan <- sizeBuf
			}
			loopCopy := loop
			utils.PanicCapturingGo(func() { f(loopCopy) })
		}

		sizeWg.Wait()
		size = 0
		for i := 0; i < numThreadsDmPointCloudAdapter; i++ {
			size += <-sizeChan
		}
	})

	wg.Wait()
	cache := pointcloud.NewWithPrealloc(size)
	return &dmPointCloudAdapter{
		dm:        newDm,
		size:      size,
		p:         p,
		cache:     cache,
		cached:    false,
		cacheLock: &sync.Mutex{},
	}
}

type dmPointCloudAdapter struct {
	dm        *DepthMap
	p         Projector
	size      int
	cache     pointcloud.PointCloud
	cached    bool
	cacheLock *sync.Mutex
}

func (dm *dmPointCloudAdapter) safeCacheSet(pt r3.Vector, d pointcloud.Data) error {
	dm.cacheLock.Lock()
	defer dm.cacheLock.Unlock()
	return dm.cache.Set(pt, d)
}

func (dm *dmPointCloudAdapter) Size() int {
	return dm.size
}

// genCache generates the cache if it is not already generated.
func (dm *dmPointCloudAdapter) genCache() {
	if dm.cached {
		return
	}
	var wg sync.WaitGroup
	wg.Add(numThreadsDmPointCloudAdapter)
	for loop := 0; loop < numThreadsDmPointCloudAdapter; loop++ {
		f := func(loop int) {
			defer wg.Done()
			// dm.Iterate automatically caches results
			dm.Iterate(numThreadsDmPointCloudAdapter, loop, func(p r3.Vector, d pointcloud.Data) bool {
				return true
			})
		}
		loopCopy := loop
		utils.PanicCapturingGo(func() { f(loopCopy) })
	}
	wg.Wait()
	dm.cached = true
}

func (dm *dmPointCloudAdapter) MetaData() pointcloud.MetaData {
	dm.genCache()
	return dm.cache.MetaData()
}

func (dm *dmPointCloudAdapter) Set(p r3.Vector, d pointcloud.Data) error {
	return errors.New("dmPointCloudAdapter Does not support Set")
}

func (dm *dmPointCloudAdapter) At(x, y, z float64) (pointcloud.Data, bool) {
	dm.genCache()
	return dm.cache.At(x, y, z)
}

func (dm *dmPointCloudAdapter) Iterate(numBatches, myBatch int, fn func(pt r3.Vector, d pointcloud.Data) bool) {
	if dm.cached {
		dm.cache.Iterate(numBatches, myBatch, fn)
		return
	}
	for y := 0; y < dm.dm.height; y++ {
		if numBatches > 0 && y%numBatches != myBatch {
			continue
		}
		for x := 0; x < dm.dm.width; x++ {
			depth := dm.dm.GetDepth(x, y)
			if depth == 0 {
				continue
			}
			vec, err := dm.p.ImagePointTo3DPoint(image.Point{x, y}, depth)
			if err != nil {
				panic(err)
			}
			if !dm.cached {
				err = dm.safeCacheSet(vec, nil)
				if err != nil {
					panic(err)
				}
			}
			if !fn(vec, nil) {
				return
			}
		}
	}
	// Since there is no orchestrator for Iterate, we need to check within each process
	if dm.size == dm.cache.Size() {
		dm.cached = true
	}
}
