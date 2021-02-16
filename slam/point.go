package slam

import (
	"image/color"
)

type Point interface {
	Position() Vec3
	Color() *color.RGBA
}

type basicPoint Vec3

func (bp basicPoint) Position() Vec3 {
	return Vec3(bp)
}

func (bp basicPoint) Color() *color.RGBA {
	return nil
}

func NewPoint(x, y, z int) Point {
	return basicPoint{x, y, z}
}

type FloatPoint interface {
	Point
	Value() float64
}

type basicFloatPoint struct {
	basicPoint
	value float64
}

func (bfp basicFloatPoint) Value() float64 {
	return bfp.value
}

func NewFloatPoint(x, y, z int, v float64) FloatPoint {
	return basicFloatPoint{basicPoint{x, y, z}, v}
}
