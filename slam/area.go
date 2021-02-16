package slam

import (
	"image"
	"sync"

	"github.com/james-bowman/sparse"
	"github.com/jblindsay/lidario"
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

func (sa *SquareArea) WriteToFile(fn string) error {
	lf, err := lidario.NewLasFile(fn, "w")
	if err != nil {
		return err
	}
	if err := lf.AddHeader(lidario.LasHeader{}); err != nil {
		return err
	}

	var lastErr error
	sa.Mutate(func(area MutableArea) {
		area.DoNonZero(func(x, y int, v float64) {
			if err := lf.AddLasPoint(&lidario.PointRecord2{
				PointRecord0: &lidario.PointRecord0{
					X:         float64(x),
					Y:         float64(y),
					Z:         0,
					Intensity: 0,
					BitField: lidario.PointBitField{
						Value: (1) | (1 << 3) | (0 << 6) | (0 << 7),
					},
					ClassBitField: lidario.ClassificationBitField{
						Value: 0,
					},
					ScanAngle:     0,
					UserData:      0,
					PointSourceID: 1,
				},
				RGB: &lidario.RgbData{
					Red: 255,
				},
			}); err != nil {
				lastErr = err
			}
		})
	})
	if lastErr != nil {
		if err := lf.Close(); err != nil {
			return err
		}
		return lastErr
	}

	return lf.Close()
}
