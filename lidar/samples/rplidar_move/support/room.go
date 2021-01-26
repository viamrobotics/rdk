package support

import (
	"image"
	"sync"

	"github.com/james-bowman/sparse"
)

func NewSquareRoom(meters int, scaleTo int) *SquareRoom {
	measurementScaled := meters * scaleTo
	points := sparse.NewDOK(measurementScaled, measurementScaled)
	centerX := measurementScaled / 2
	centerY := centerX

	return &SquareRoom{
		sizeMeters: meters,
		scale:      scaleTo,
		points:     points,
		centerX:    centerX,
		centerY:    centerY,
	}
}

type SquareRoom struct {
	mu         sync.Mutex
	sizeMeters int
	scale      int
	points     *sparse.DOK
	centerX    int
	centerY    int
}

func (sr *SquareRoom) Size() (int, int) {
	return sr.sizeMeters, sr.scale
}

func (sr *SquareRoom) Center() image.Point {
	return image.Point{sr.centerX, sr.centerY}
}

type MutableRoom interface {
	DoNonZero(visit func(x, y int, v float64))
	Set(x, y int, v float64)
}

func (sr *SquareRoom) Mutate(mutator func(room MutableRoom)) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	mutator(sr.points)
}
