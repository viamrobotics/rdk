package rimage

import (
	"image"
	"sync"

	"github.com/golang/geo/r3"
	"go.viam.com/utils"

	"go.viam.com/rdk/pointcloud"
)

func newDMPointCloudAdapter(dm *DepthMap, p Projector) *dmPointCloudAdapter {
	var wg sync.WaitGroup
	wg.Add(2)
	var newDm *DepthMap
	utils.PanicCapturingGo(func() {
		defer wg.Done()
		newDm = dm.Clone()
	})

	size := 0
	utils.PanicCapturingGo(func() {
		defer wg.Done()
		for x := 0; x < dm.Width(); x++ {
			for y := 0; y < dm.Height(); y++ {
				z := dm.GetDepth(x, y)
				if z == 0 {
					continue
				}
				size++
			}
		}
	})

	wg.Wait()
	return &dmPointCloudAdapter{
		dm:     newDm,
		size:   size,
		p:      p,
		cache:  pointcloud.NewWithPrealloc(size),
		cached: false,
	}
}

type dmPointCloudAdapter struct {
	dm     *DepthMap
	p      Projector
	size   int
	cache  pointcloud.PointCloud
	cached bool
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
	const numLoops = 8
	wg.Add(numLoops)
	for loop := 0; loop < numLoops; loop++ {
		f := func(loop int) {
			defer wg.Done()
			// dm.Iterate automatically caches results
			dm.Iterate(numLoops, loop, func(p r3.Vector, d pointcloud.Data) bool {
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
	dm.genCache()
	return dm.cache.Set(p, d)
}

func (dm *dmPointCloudAdapter) At(x, y, z float64) (pointcloud.Data, bool) {
	dm.genCache()
	return dm.cache.At(x, y, z)
}

func (dm *dmPointCloudAdapter) Iterate(numBatches, myBatch int, fn func(pt r3.Vector, d pointcloud.Data) bool) {
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
				err = dm.cache.Set(vec, nil)
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
