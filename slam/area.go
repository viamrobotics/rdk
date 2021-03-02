package slam

import (
	"fmt"
	"image"
	"sync"

	"go.viam.com/robotcore/pointcloud"
)

// TODO(erd): adapt to use float64 on points, if it makes sense.
// If it does not make sense, then reason how to resolve duplicate
// points in the cloud at the same X or Y.
func NewSquareArea(sizeMeters int, scaleTo int) *SquareArea {
	cloud := pointcloud.New()
	return SquareAreaFromPointCloud(cloud, sizeMeters, scaleTo)
}

func NewSquareAreaFromFile(fn string, sizeMeters int, scaleTo int) (*SquareArea, error) {
	cloud, err := pointcloud.NewFromFile(fn)
	if err != nil {
		return nil, err
	}
	return SquareAreaFromPointCloud(cloud, sizeMeters, scaleTo), nil
}

func SquareAreaFromPointCloud(cloud *pointcloud.PointCloud, sizeMeters int, scaleTo int) *SquareArea {
	measurementScaled := sizeMeters * scaleTo
	centerX := measurementScaled / 2
	centerY := centerX

	return &SquareArea{
		sizeMeters: sizeMeters,
		scaleTo:    scaleTo,
		dim:        sizeMeters * scaleTo,
		cloud:      cloud,
		centerX:    centerX,
		centerY:    centerY,
	}
}

type SquareArea struct {
	mu         sync.Mutex
	sizeMeters int
	scaleTo    int
	dim        int
	cloud      *pointcloud.PointCloud
	centerX    int
	centerY    int
}

func (sa *SquareArea) BlankCopy() *SquareArea {
	return NewSquareArea(sa.sizeMeters, sa.scaleTo)
}

func (sa *SquareArea) Size() (int, int) {
	return sa.sizeMeters, sa.scaleTo
}

func (sa *SquareArea) Dims() (int, int) {
	return sa.dim, sa.dim
}

func (sa *SquareArea) Center() image.Point {
	return image.Point{sa.centerX, sa.centerY}
}

func (sa *SquareArea) WriteToFile(fn string) error {
	return sa.cloud.WriteToFile(fn)
}

type MutableArea interface {
	Iterate(visit func(x, y, v int) bool)
	At(x, y int) int
	Set(x, y int, v int)
	Unset(x, y int)
}

func (sa *SquareArea) Mutate(mutator func(room MutableArea)) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	mutator((*mutableSquareArea)(sa))
}

type mutableSquareArea SquareArea

func (msa *mutableSquareArea) Iterate(visit func(x, y, v int) bool) {
	msa.cloud.Iterate(func(p pointcloud.Point) bool {
		pos := p.Position()
		return visit(pos.X, pos.Y, p.(pointcloud.ValuePoint).Value())
	})
}

func (msa *mutableSquareArea) At(x, y int) int {
	p := msa.cloud.At(x, y, 0)
	if p == nil {
		return 0
	}
	return p.(pointcloud.ValuePoint).Value()
}

func (msa *mutableSquareArea) Set(x, y, v int) {
	if x < 0 || x >= msa.dim {
		panic(fmt.Errorf("x must be between [0,%d)", msa.dim))
	}
	if y < 0 || y >= msa.dim {
		panic(fmt.Errorf("y must be between [0,%d)", msa.dim))
	}
	msa.cloud.Set(pointcloud.NewValuePoint(x, y, 0, v))
}

func (msa *mutableSquareArea) Unset(x, y int) {
	msa.cloud.Unset(x, y, 0)
}
