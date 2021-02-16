package pc

import (
	"image/color"
)

type Point interface {
	Position() Vec3
}

func NewPoint(x, y, z int) Point {
	return basicPoint{x, y, z}
}

type basicPoint Vec3

func (bp basicPoint) Position() Vec3 {
	return Vec3(bp)
}

type FloatPoint interface {
	Point
	Value() float64
}

func NewFloatPoint(x, y, z int, v float64) FloatPoint {
	return basicFloatPoint{basicPoint{x, y, z}, v}
}

type basicFloatPoint struct {
	basicPoint
	value float64
}

func (bfp basicFloatPoint) Value() float64 {
	return bfp.value
}

type ColoredPoint interface {
	Point
	Color() *color.RGBA
}

func NewColoredPoint(x, y, z int, c *color.RGBA) ColoredPoint {
	return basicColoredPoint{basicPoint{x, y, z}, c}
}

type basicColoredPoint struct {
	basicPoint
	c *color.RGBA
}

func (bcp basicColoredPoint) Color() *color.RGBA {
	return bcp.c
}
