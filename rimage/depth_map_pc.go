package rimage

import (
	"errors"
	"image"
	"sync"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/utils"
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
		dm:   newDm,
		size: size,
		p:    p,
	}
}

type dmPointCloudAdapter struct {
	dm   *DepthMap
	p    Projector
	size int
}

func (dm *dmPointCloudAdapter) Size() int {
	return dm.size
}

func (dm *dmPointCloudAdapter) MetaData() pointcloud.MetaData {
	panic(1)
}

func (dm *dmPointCloudAdapter) Set(p r3.Vector, d pointcloud.Data) error {
	return errors.New("dmPointCloudAdapter doesn't support Set")
}

func (dm *dmPointCloudAdapter) At(x, y, z float64) (pointcloud.Data, bool) {
	panic(7)
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
			if !fn(vec, nil) {
				return
			}
		}
	}
}
