package slam

import (
	"image"
	"sync"

	"github.com/james-bowman/sparse"
)

func NewSquareArea(meters int, scaleTo int) *SquareArea {
	measurementScaled := meters * scaleTo
	points := sparse.NewDOK(measurementScaled, measurementScaled)
	centerX := measurementScaled / 2
	centerY := centerX

	return &SquareArea{
		sizeMeters: meters,
		scale:      scaleTo,
		points:     points,
		centerX:    centerX,
		centerY:    centerY,
	}
}

type SquareArea struct {
	mu         sync.Mutex
	sizeMeters int
	scale      int
	points     *sparse.DOK
	centerX    int
	centerY    int
}

func (sa *SquareArea) Size() (int, int) {
	return sa.sizeMeters, sa.scale
}

func (sa *SquareArea) Center() image.Point {
	return image.Point{sa.centerX, sa.centerY}
}

type MutableArea interface {
	DoNonZero(visit func(x, y int, v float64))
	At(x, y int) float64
	Set(x, y int, v float64)
}

func (sa *SquareArea) Mutate(mutator func(room MutableArea)) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	mutator(sa.points)
}
