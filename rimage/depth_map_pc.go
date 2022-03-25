package rimage

import (
	"errors"
	"image"
	"io"

	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/utils"
)

type dmPointCloudAdapter struct {
	dm *DepthMap
	p  Projector
}

func (dm *dmPointCloudAdapter) Size() int {
	return dm.dm.width * dm.dm.height
}

func (dm *dmPointCloudAdapter) HasColor() bool {
	return false
}

func (dm *dmPointCloudAdapter) HasValue() bool {
	return false
}

func (dm *dmPointCloudAdapter) MinX() float64 {
	panic(1)
}

func (dm *dmPointCloudAdapter) MaxX() float64 {
	panic(2)
}

func (dm *dmPointCloudAdapter) MinY() float64 {
	panic(3)
}

func (dm *dmPointCloudAdapter) MaxY() float64 {
	panic(4)
}

func (dm *dmPointCloudAdapter) MinZ() float64 {
	panic(5)
}

func (dm *dmPointCloudAdapter) MaxZ() float64 {
	panic(6)
}

func (dm *dmPointCloudAdapter) Set(p pointcloud.Point) error {
	return errors.New("dmPointCloudAdapter doesn't support Set")
}

func (dm *dmPointCloudAdapter) Unset(x, y, z float64) {
	panic("dmPointCloudAdapter doesn't support Unset")
}

func (dm *dmPointCloudAdapter) At(x, y, z float64) pointcloud.Point {
	panic(7)
}

func (dm *dmPointCloudAdapter) Iterate(fn func(p pointcloud.Point) bool) {
	for y := 0; y < dm.dm.height; y++ {
		for x := 0; x < dm.dm.width; x++ {
			vec, err := dm.p.ImagePointTo3DPoint(image.Point{x, y}, dm.dm.GetDepth(x, y))
			if err != nil {
				panic(err)
			}
			if !fn(pointcloud.NewJustAPoiint(pointcloud.Vec3(vec))) {
				return
			}
		}
	}
}

func (dm *dmPointCloudAdapter) WriteToFile(fn string) error {
	return errors.New("dmPointCloudAdapter doesn't support WriteToFile")
}

func (dm *dmPointCloudAdapter) ToPCD(out io.Writer, outputType pointcloud.PCDType) error {
	return errors.New("dmPointCloudAdapter doesn't support ToPCD")
}

func (dm *dmPointCloudAdapter) DenseZ(zIdx float64) (*mat.Dense, error) {
	return nil, errors.New("dmPointCloudAdapter doesn't support DenseZ")
}

func (dm *dmPointCloudAdapter) ToVec2Matrix() (*utils.Vec2Matrix, error) {
	return nil, errors.New("dmPointCloudAdapter doesn't support ToVec2Matrix")
}

func (dm *dmPointCloudAdapter) Points() []pointcloud.Point {
	panic(8)
}
