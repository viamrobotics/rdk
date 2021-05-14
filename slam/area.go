package slam

import (
	"sync"

	"github.com/go-errors/errors"

	"github.com/edaniels/golog"

	"go.viam.com/core/pointcloud"
)

// SquareArea is the map that is used to record points detected with SLAM.
type SquareArea struct {
	mu            sync.Mutex
	sizeMeters    float64
	unitsPerMeter float64
	dim           float64
	quadLength    float64
	cloud         pointcloud.PointCloud
}

// NewSquareArea returns a square area that has a given size in a particular unit.
func NewSquareArea(sizeMeters float64, unitsPerMeter float64, logger golog.Logger) (*SquareArea, error) {
	cloud := pointcloud.NewRoundingPointCloud()
	return SquareAreaFromPointCloud(cloud, sizeMeters, unitsPerMeter)
}

// NewSquareAreaFromFile returns a square area that is read in from a point cloud file.
func NewSquareAreaFromFile(fn string, sizeMeters float64, unitsPerMeter float64, logger golog.Logger) (*SquareArea, error) {
	cloud, err := pointcloud.NewRoundingPointCloudFromFile(fn, logger)
	if err != nil {
		return nil, err
	}
	return SquareAreaFromPointCloud(cloud, sizeMeters, unitsPerMeter)
}

// SquareAreaFromPointCloud transforms a point cloud into a suitable square area.
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

// PointCloud returns the mutable PointCloud this area uses
// to store its points.
func (sa *SquareArea) PointCloud() pointcloud.PointCloud {
	return sa.cloud
}

// BlankCopy returns a new, empty square area that has the same dimensions.
func (sa *SquareArea) BlankCopy(logger golog.Logger) (*SquareArea, error) {
	area, err := NewSquareArea(sa.sizeMeters, sa.unitsPerMeter, logger)
	if err != nil {
		return nil, err
	}
	return area, nil
}

// Size returns the size of the area in meters along with its units.
func (sa *SquareArea) Size() (float64, float64) {
	return sa.sizeMeters, sa.unitsPerMeter
}

// Dim returns a dimension size of the square in units.
func (sa *SquareArea) Dim() float64 {
	return sa.dim
}

// QuadrantLength returns the length of a quadrant (1/4 of the square) in units.
func (sa *SquareArea) QuadrantLength() float64 {
	return sa.quadLength
}

// WriteToFile writes the area into a file as a point cloud.
func (sa *SquareArea) WriteToFile(fn string) error {
	return sa.cloud.WriteToFile(fn)
}

// A MutableArea allows for an area to be safely mutated.
type MutableArea interface {
	// Iterate visits each set point in the area and calls the given visitor function.
	Iterate(visit func(x, y float64, v int) bool)

	// At returns the value at a particular point. If the point is not present,
	// 0 is returned as there can be no zero values.
	At(x, y float64) int

	// Set adds a point into the area. If a point is already present,
	// this overwrites it.
	Set(x, y float64, v int) error

	// Unset removes a point from the area. If it's not present, this
	// does nothing.
	Unset(x, y float64)
}

// Mutate prepares this area for mutation by one caller at a time.
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
		return errors.Errorf("x must be between [%v,%v)", -msa.quadLength, msa.quadLength)
	}
	if y < -msa.quadLength || y >= msa.quadLength {
		return errors.Errorf("y must be between [%v,%v)", -msa.quadLength, msa.quadLength)
	}
	return msa.cloud.Set(pointcloud.NewValuePoint(x, y, 0, v))
}

func (msa *mutableSquareArea) Unset(x, y float64) {
	msa.cloud.Unset(x, y, 0)
}
