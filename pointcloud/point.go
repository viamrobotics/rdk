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
type Data interface {
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
	SetColor(c color.NRGBA) Data

	// HasValue returns whether or not this point has some user data value
	// associated with it.
	HasValue() bool

	// Value returns the user data set value, if it exists.
	Value() int

	// SetValue sets the given user data value on the point.
	// Note(erd): we should try to remove this in favor of immutability.
	SetValue(v int) Data

	// Value returns the intesity value, or 0 if it doesn't exist
	Intensity() uint16

	// SetIntensity sets the intensity on the point.
	SetIntensity(v uint16) Data
}

type basicData struct {
	hasColor bool
	c        color.NRGBA

	hasValue bool
	value    int

	intensity uint16
}

// NewBasicPoint returns a point that is solely positionally based.
func NewBasicData() Data {
	return &basicData{}
}

// NewColoredPoint returns a point that has both position and color.
func NewColoredData(c color.NRGBA) Data {
	return &basicData{c: c, hasColor: true}
}

// NewValuePoint returns a point that has both position and a user data value.
func NewValueData(v int) Data {
	return &basicData{value: v, hasValue: true}
}

func (bp *basicData) HasColor() bool {
	return bp.hasColor
}

func (bp *basicData) RGB255() (uint8, uint8, uint8) {
	return bp.c.R, bp.c.G, bp.c.B
}

func (bp *basicData) Color() color.Color {
	return &bp.c
}

func (bp *basicData) SetValue(v int) Data {
	bp.hasValue = true
	bp.value = v
	return bp
}

func (bp *basicData) HasValue() bool {
	return bp.hasValue
}

func (bp *basicData) Value() int {
	return bp.value
}

func (bp *basicData) SetIntensity(v uint16) Data {
	bp.intensity = v
	return bp
}

func (bp *basicData) Intensity() uint16 {
	return bp.intensity
}

