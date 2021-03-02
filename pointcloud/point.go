package pointcloud

import (
	"image/color"
)

type Point interface {
	Position() Vec3
}

func NewPoint(x, y, z int) Point {
	return basicPoint{x, y, z}
}

func IsValue(p Point) (bool, ValuePoint) {
	switch v := p.(type) {
	case basicPoint:
		return false, nil
	case basicValuePoint:
		return true, v
	case basicColoredPoint:
		return IsValue(v.Point)
	default:
		return false, nil
	}
}

func IsColored(p Point) (bool, ColoredPoint) {
	switch v := p.(type) {
	case basicPoint:
		return false, nil
	case basicValuePoint:
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

type ValuePoint interface {
	Point
	Value() int
}

func NewValuePoint(x, y, z int, v int) ValuePoint {
	return basicValuePoint{basicPoint{x, y, z}, v}
}

func WithPointValue(p Point, v int) ValuePoint {
	return basicValuePoint{p, v}
}

func WithPointColor(p Point, c *color.RGBA) ColoredPoint {
	return basicColoredPoint{p, c}
}

type basicValuePoint struct {
	Point
	value int
}

func (bvp basicValuePoint) Value() int {
	return bvp.value
}

type ColoredPoint interface {
	Point
	Color() *color.RGBA
}

func NewColoredPoint(x, y, z int, c *color.RGBA) ColoredPoint {
	return basicColoredPoint{basicPoint{x, y, z}, c}
}

type basicColoredPoint struct {
	Point
	c *color.RGBA
}

func (bcp basicColoredPoint) Color() *color.RGBA {
	return bcp.c
}
