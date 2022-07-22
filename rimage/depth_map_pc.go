package rimage

import (
	"errors"
	"image"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/pointcloud"
)

func newDMPointCloudAdapter(dm *DepthMap, p Projector) *dmPointCloudAdapter {
	pc := &dmPointCloudAdapter{
		dm: dm,
		p:  p,
	}

	for x := 0; x < dm.Width(); x++ {
		for y := 0; y < dm.Height(); y++ {
			z := dm.GetDepth(x, y)
			if z == 0 {
				continue
			}
			pc.size++
		}
	}

	return pc
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
