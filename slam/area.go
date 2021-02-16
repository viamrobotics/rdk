package slam

import (
	"image"
	"math"
	"sync"

	"github.com/viamrobotics/robotcore/pc"
)

func NewSquareArea(meters int, scaleTo int) *SquareArea {
	measurementScaled := meters * scaleTo
	pc := pc.NewPointCloud()
	centerX := measurementScaled / 2
	centerY := centerX

	return &SquareArea{
		sizeMeters: meters,
		scale:      scaleTo,
		pc:         pc,
		centerX:    centerX,
		centerY:    centerY,
	}
}

type SquareArea struct {
	mu         sync.Mutex
	sizeMeters int
	scale      int
	pc         *pc.PointCloud
	centerX    int
	centerY    int
}

func (sa *SquareArea) Size() (int, int) {
	return sa.sizeMeters, sa.scale
}

func (sa *SquareArea) Center() image.Point {
	return image.Point{sa.centerX, sa.centerY}
}

func (sa *SquareArea) WriteToFile(fn string) error {
	return sa.pc.WriteToFile(fn)
}

type MutableArea interface {
	Iterate(visit func(x, y int, v float64) bool)
	At(x, y int) float64
	Set(x, y int, v float64)
	Unset(x, y int)
}

func (sa *SquareArea) Mutate(mutator func(room MutableArea)) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	mutator((*mutableSquareArea)(sa))
}

type mutableSquareArea SquareArea

func (msa *mutableSquareArea) Iterate(visit func(x, y int, v float64) bool) {
	msa.pc.Iterate(func(p pc.Point) bool {
		pos := p.Position()
		return visit(pos.X, pos.Y, p.(pc.FloatPoint).Value())
	})
}

func (msa *mutableSquareArea) At(x, y int) float64 {
	p := msa.pc.At(x, y, 0)
	if p == nil {
		return math.NaN()
	}
	return p.(pc.FloatPoint).Value()
}

func (msa *mutableSquareArea) Set(x, y int, v float64) {
	msa.pc.Set(pc.NewFloatPoint(x, y, 0, v))
}

func (msa *mutableSquareArea) Unset(x, y int) {
	msa.pc.Unset(x, y, 0)
}
