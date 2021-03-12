package slam

import (
	"errors"
	"fmt"
	"sync"

	"go.viam.com/robotcore/pointcloud"
)

func NewSquareArea(sizeMeters int, unitsPerMeter int) (*SquareArea, error) {
	cloud := pointcloud.New()
	return SquareAreaFromPointCloud(cloud, sizeMeters, unitsPerMeter)
}

func NewSquareAreaFromFile(fn string, sizeMeters int, unitsPerMeter int) (*SquareArea, error) {
	cloud, err := pointcloud.NewFromFile(fn)
	if err != nil {
		return nil, err
	}
	return SquareAreaFromPointCloud(cloud, sizeMeters, unitsPerMeter)
}

func SquareAreaFromPointCloud(cloud *pointcloud.PointCloud, sizeMeters int, unitsPerMeter int) (*SquareArea, error) {
	sizeUnits := sizeMeters * unitsPerMeter
	if sizeUnits%2 != 0 {
		return nil, errors.New("sizeMeters * unitsPerMeter must be divisible by 2")
	}

	return &SquareArea{
		sizeMeters:    sizeMeters,
		unitsPerMeter: unitsPerMeter,
		dim:           sizeUnits,
		quadLength:    sizeUnits / 2,
		cloud:         cloud,
	}, nil
}

type SquareArea struct {
	mu            sync.Mutex
	sizeMeters    int
	unitsPerMeter int
	dim           int
	quadLength    int
	cloud         *pointcloud.PointCloud
}

// PointCloud returns the mutable PointCloud this area uses
// to store its points.
func (sa *SquareArea) PointCloud() *pointcloud.PointCloud {
	return sa.cloud
}

func (sa *SquareArea) BlankCopy() (*SquareArea, error) {
	area, err := NewSquareArea(sa.sizeMeters, sa.unitsPerMeter)
	if err != nil {
		return nil, err
	}
	return area, nil
}

func (sa *SquareArea) Size() (int, int) {
	return sa.sizeMeters, sa.unitsPerMeter
}

func (sa *SquareArea) Dim() int {
	return sa.dim
}

func (sa *SquareArea) QuadrantLength() int {
	return sa.quadLength
}

func (sa *SquareArea) WriteToFile(fn string) error {
	return sa.cloud.WriteToFile(fn)
}

type MutableArea interface {
	Iterate(visit func(x, y, v int) bool)
	At(x, y int) int
	Set(x, y int, v int) error
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
		return visit(pos.X, pos.Y, p.Value())
	})
}

func (msa *mutableSquareArea) At(x, y int) int {
	p := msa.cloud.At(x, y, 0)
	if p == nil {
		return 0
	}
	return p.Value()
}

func (msa *mutableSquareArea) Set(x, y, v int) error {
	if x < -msa.quadLength || x >= msa.quadLength {
		return fmt.Errorf("x must be between [%d,%d)", -msa.quadLength, msa.quadLength)
	}
	if y < -msa.quadLength || y >= msa.quadLength {
		return fmt.Errorf("y must be between [%d,%d)", -msa.quadLength, msa.quadLength)
	}
	return msa.cloud.Set(pointcloud.NewValuePoint(x, y, 0, v))
}

func (msa *mutableSquareArea) Unset(x, y int) {
	msa.cloud.Unset(x, y, 0)
}
