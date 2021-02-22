package pc

import (
	"image/color"
)

type Point interface {
	Position() Vec3
}

func NewPoint(x, y, z float64) Point {
	return basicPoint{x, y, z}
}

func IsFloat(p Point) (bool, FloatPoint) {
	switch v := p.(type) {
	case basicPoint:
		return false, nil
	case basicFloatPoint:
		return true, v
	case basicColoredPoint:
		return IsFloat(v.Point)
	default:
		return false, nil
	}
}

func IsColored(p Point) (bool, ColoredPoint) {
	switch v := p.(type) {
	case basicPoint:
		return false, nil
	case basicFloatPoint:
		return IsColored(v.Point)
	case basicColoredPoint:
		return true, v
	default:
		return false, nil
	}
}

type basicPoint Vec3

func (bp basicPoint) Position() Vec3 {
	return Vec3(bp)
}

type FloatPoint interface {
	Point
	Value() float64
}

func NewFloatPoint(x, y, z float64, v float64) FloatPoint {
	return basicFloatPoint{basicPoint{x, y, z}, v}
}

func WithPointValue(p Point, v float64) FloatPoint {
	return basicFloatPoint{p, v}
}

func WithPointColor(p Point, c *color.RGBA) ColoredPoint {
	return basicColoredPoint{p, c}
}

type basicFloatPoint struct {
	Point
	value float64
}

func (bfp basicFloatPoint) Value() float64 {
	return bfp.value
}

type ColoredPoint interface {
	Point
	Color() *color.RGBA
}

func NewColoredPoint(x, y, z float64, c *color.RGBA) ColoredPoint {
	return basicColoredPoint{basicPoint{x, y, z}, c}
}

type basicColoredPoint struct {
	Point
	c *color.RGBA
}

func (bcp basicColoredPoint) Color() *color.RGBA {
	return bcp.c
}
