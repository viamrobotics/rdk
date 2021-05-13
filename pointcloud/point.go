package pointcloud

import (
	"image/color"

	"github.com/golang/geo/r3"
)

// Vec3 is a three-dimensional vector.
type Vec3 r3.Vector

// Vec3s is a series of three-dimensional vectors.
type Vec3s []Vec3

// Len returns the number of vectors.
func (vs Vec3s) Len() int {
	return len(vs)
}

// Swap swaps two vectors positionally.
func (vs Vec3s) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}

// Less returns which vector is less than the other based on
// r3.Vector.Cmp.
func (vs Vec3s) Less(i, j int) bool {
	cmp := (r3.Vector)(vs[i]).Cmp((r3.Vector)(vs[j]))
	if cmp == 0 {
		return false
	}
	return cmp < 0
}

// A Point describes a single point within a PointCloud. It is the
// collection of these points that forms the cloud.
type Point interface {
	// Position is the vector describing where the point is in the cloud.
	Position() Vec3

	// SetPosition moves the point to the given position.
	// Note(erd): we should try to remove this in favor of immutability.
	SetPosition(p Vec3)

	// HasColor returns whether or not this point is colored.
	HasColor() bool

	// RGB255 returns, if colored, the RGB components of the color. There
	// is no alpha channel right now and as such the data can be assumed to be
	// premultiplied.
	RGB255() (uint8, uint8, uint8)

	// Color returns the native color of the point.
	Color() color.Color

	// SetColor sets the given color on the point.
	// Note(erd): we should try to remove this in favor of immutability.
	SetColor(c color.NRGBA) Point

	// HasValue returns whether or not this point has some user data value
	// associated with it.
	HasValue() bool

	// Value returns the user data set value, if it exists.
	Value() int

	// SetValue sets the given user data value on the point.
	// Note(erd): we should try to remove this in favor of immutability.
	SetValue(v int) Point
}

type basicPoint struct {
	position Vec3

	hasColor bool
	c        color.NRGBA

	hasValue bool
	value    int
}

// NewBasicPoint returns a point that is solely positionally based.
func NewBasicPoint(x, y, z float64) Point {
	return &basicPoint{position: Vec3{x, y, z}}
}

// NewColoredPoint returns a point that has both position and color.
func NewColoredPoint(x, y, z float64, c color.NRGBA) Point {
	return &basicPoint{position: Vec3{x, y, z}, c: c, hasColor: true}
}

// NewValuePoint returns a point that has both position and a user data value.
func NewValuePoint(x, y, z float64, v int) Point {
	return &basicPoint{position: Vec3{x, y, z}, value: v, hasValue: true}
}

func (bp *basicPoint) Position() Vec3 {
	return bp.position
}

func (bp *basicPoint) SetPosition(p Vec3) {
	bp.position = p
}

func (bp *basicPoint) SetColor(c color.NRGBA) Point {
	bp.hasColor = true
	bp.c = c
	return bp
}

func (bp *basicPoint) HasColor() bool {
	return bp.hasColor
}

func (bp *basicPoint) RGB255() (uint8, uint8, uint8) {
	return bp.c.R, bp.c.G, bp.c.B
}

func (bp *basicPoint) Color() color.Color {
	return &bp.c
}

func (bp *basicPoint) SetValue(v int) Point {
	bp.hasValue = true
	bp.value = v
	return bp
}

func (bp *basicPoint) HasValue() bool {
	return bp.hasValue
}

func (bp *basicPoint) Value() int {
	return bp.value
}
