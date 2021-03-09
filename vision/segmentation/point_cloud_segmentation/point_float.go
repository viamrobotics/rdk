package point_cloud_segmentation
import (
	"image/color"
	"github.com/golang/geo/r3"
)

type PointFloat interface {
	Position() r3.Vector
}

func NewPointFloat(x, y, z float64) PointFloat {
	return basicPointFloat{x, y, z}
}


func IsValue(p PointFloat) (bool, ValuePointFloat) {
	switch v := p.(type) {
	case basicPointFloat:
		return false, nil
	case basicValuePointFloat:
		return true, v
	case basicColoredPointFloat:
		return IsValue(v.PointFloat)
	default:
		return false, nil
	}
}

func IsColored(p PointFloat) (bool, ColoredPointFloat) {
	switch v := p.(type) {
	case basicPointFloat:
		return false, nil
	case basicValuePointFloat:
		return IsColored(v.PointFloat)
	case basicColoredPointFloat:
		return true, v
	default:
		return false, nil
	}
}


type basicPointFloat r3.Vector

func (bp basicPointFloat) Position() r3.Vector {
	return r3.Vector(bp)
}

type ValuePointFloat interface {
	PointFloat
	Value() int
}

func NewValuePointFloat(x, y, z float64, v int) ValuePointFloat {
	return basicValuePointFloat{basicPointFloat{x, y, z}, v}
}

func WithPointValueFloat(p PointFloat, v int) ValuePointFloat {
	return basicValuePointFloat{p, v}
}

func WithPointColorFloat(p PointFloat, c *color.RGBA) ColoredPointFloat {
	return basicColoredPointFloat{p, c}
}

type basicValuePointFloat struct {
	PointFloat
	value int
}

func (bvp basicValuePointFloat) Value() int {
	return bvp.value
}

type ColoredPointFloat interface {
	PointFloat
	Color() *color.RGBA
}

func NewColoredPointFloat(x, y, z float64, c *color.RGBA) ColoredPointFloat {
	return basicColoredPointFloat{basicPointFloat{x, y, z}, c}
}

type basicColoredPointFloat struct {
	PointFloat
	c *color.RGBA
}

func (bcp basicColoredPointFloat) Color() *color.RGBA {
	return bcp.c
}
