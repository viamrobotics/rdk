package slam

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/pointcloud"
)

func NewSquareArea(sizeMeters float64, unitsPerMeter float64, logger golog.Logger) (*SquareArea, error) {
	storage := NewRoundingPointCloud()
	return SquareAreaFromPointStorage(storage, sizeMeters, unitsPerMeter)
}

func NewSquareAreaFromFile(fn string, sizeMeters float64, unitsPerMeter float64, logger golog.Logger) (*SquareArea, error) {
	storage, err := NewRoundingPointCloudFromFile(fn, logger)
	if err != nil {
		return nil, err
	}
	return SquareAreaFromPointStorage(storage, sizeMeters, unitsPerMeter)
}

func SquareAreaFromPointStorage(storage PointStorage, sizeMeters float64, unitsPerMeter float64) (*SquareArea, error) {
	sizeUnits := sizeMeters * unitsPerMeter
	if int(math.Round(sizeUnits))%2 != 0 {
		return nil, errors.New("sizeMeters * unitsPerMeter must be divisible by 2")
	}

	return &SquareArea{
		sizeMeters:    sizeMeters,
		unitsPerMeter: unitsPerMeter,
		dim:           sizeUnits,
		quadLength:    sizeUnits / 2,
		storage:       storage,
	}, nil
}

type SquareArea struct {
	mu            sync.Mutex
	sizeMeters    float64
	unitsPerMeter float64
	dim           float64
	quadLength    float64
	storage       PointStorage
}

// PointStorage returns the mutable PointStorage this area uses
// to store its points.
func (sa *SquareArea) PointStorage() PointStorage {
	return sa.storage
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
	return sa.storage.WriteToFile(fn)
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
	msa.storage.Iterate(func(p pointcloud.Point) bool {
		pos := p.Position()
		return visit(pos.X, pos.Y, p.Value())
	})
}

func (msa *mutableSquareArea) At(x, y float64) int {
	p := msa.storage.At(x, y)
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
	return msa.storage.Set(x, y, v)
}

func (msa *mutableSquareArea) Unset(x, y float64) {
	msa.storage.Unset(x, y)
}
