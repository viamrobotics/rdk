package slam

import (
	"errors"
	"fmt"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/pointcloud"
)

func NewSquareArea(sizeMeters float64, unitsPerMeter float64, logger golog.Logger) (*SquareArea, error) {
	cloud := pointcloud.NewRoundingPointCloud()
	return SquareAreaFromPointCloud(cloud, sizeMeters, unitsPerMeter)
}

func NewSquareAreaFromFile(fn string, sizeMeters float64, unitsPerMeter float64, logger golog.Logger) (*SquareArea, error) {
	cloud, err := pointcloud.NewRoundingPointCloudFromFile(fn, logger)
	if err != nil {
		return nil, err
	}
	return SquareAreaFromPointCloud(cloud, sizeMeters, unitsPerMeter)
}

func SquareAreaFromPointCloud(cloud pointcloud.PointCloud, sizeMeters float64, unitsPerMeter float64) (*SquareArea, error) {
	sizeUnits := sizeMeters * unitsPerMeter
	if int(sizeUnits)%2 != 0 {
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
	sizeMeters    float64
	unitsPerMeter float64
	dim           float64
	quadLength    float64
	cloud         pointcloud.PointCloud
}

// PointCloud returns the mutable PointCloud this area uses
// to store its points.
func (sa *SquareArea) PointCloud() pointcloud.PointCloud {
	return sa.cloud
}

func (sa *SquareArea) BlankCopy(logger golog.Logger) (*SquareArea, error) {
	area, err := NewSquareArea(sa.sizeMeters, sa.unitsPerMeter, logger)
	if err != nil {
		return nil, err
	}
	return area, nil
}

func (sa *SquareArea) Size() (float64, float64) {
	return sa.sizeMeters, sa.unitsPerMeter
}

func (sa *SquareArea) Dim() float64 {
	return sa.dim
}

func (sa *SquareArea) QuadrantLength() float64 {
	return sa.quadLength
}

func (sa *SquareArea) WriteToFile(fn string) error {
	return sa.cloud.WriteToFile(fn)
}

type MutableArea interface {
	Iterate(visit func(x, y float64, v int) bool)
	At(x, y float64) int
	Set(x, y float64, v int) error
	Unset(x, y float64)
}

func (sa *SquareArea) Mutate(mutator func(room MutableArea)) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	mutator((*mutableSquareArea)(sa))
}

type mutableSquareArea SquareArea

func (msa *mutableSquareArea) Iterate(visit func(x, y float64, v int) bool) {
	msa.cloud.Iterate(func(p pointcloud.Point) bool {
		pos := p.Position()
		return visit(pos.X, pos.Y, p.Value())
	})
}

func (msa *mutableSquareArea) At(x, y float64) int {
	p := msa.cloud.At(x, y, 0)
	if p == nil {
		return 0
	}
	return p.Value()
}

func (msa *mutableSquareArea) Set(x, y float64, v int) error {
	if x < -msa.quadLength || x >= msa.quadLength {
		return fmt.Errorf("x must be between [%v,%v)", -msa.quadLength, msa.quadLength)
	}
	if y < -msa.quadLength || y >= msa.quadLength {
		return fmt.Errorf("y must be between [%v,%v)", -msa.quadLength, msa.quadLength)
	}
	return msa.cloud.Set(pointcloud.NewValuePoint(x, y, 0, v))
}

func (msa *mutableSquareArea) Unset(x, y float64) {
	msa.cloud.Unset(x, y, 0)
}
