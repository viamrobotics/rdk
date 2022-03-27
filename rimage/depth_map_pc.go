package rimage

import (
	"errors"
	"image"

	"github.com/golang/geo/r3"
	
	"go.viam.com/rdk/pointcloud"
)

type dmPointCloudAdapter struct {
	dm *DepthMap
	p  Projector
}

func (dm *dmPointCloudAdapter) Size() int {
	return dm.dm.width * dm.dm.height
}

func (dm *dmPointCloudAdapter) MetaData() pointcloud.PointCloudMetaData {
	panic(1)
}

func (dm *dmPointCloudAdapter) Set(p r3.Vector, d pointcloud.Data) error {
	return errors.New("dmPointCloudAdapter doesn't support Set")
}

func (dm *dmPointCloudAdapter) Unset(x, y, z float64) {
	panic("dmPointCloudAdapter doesn't support Unset")
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
			vec, err := dm.p.ImagePointTo3DPoint(image.Point{x, y}, dm.dm.GetDepth(x, y))
			if err != nil {
				panic(err)
			}
			if !fn(vec, nil) {
				return
			}
		}
	}
}

