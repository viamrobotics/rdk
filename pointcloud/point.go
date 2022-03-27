package pointcloud

import (
	"image/color"
	"math"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/spatial/kdtree"
)

// NewVector convenience method for creating a vector.
func NewVector(x, y, z float64) r3.Vector {
	return r3.Vector{x, y, z}
}

// Vectors is a series of three-dimensional vectors.
type Vectors []r3.Vector

// Len returns the number of vectors.
func (vs Vectors) Len() int {
	return len(vs)
}

// Swap swaps two vectors positionally.
func (vs Vectors) Swap(i, j int) {
	vs[i], vs[j] = vs[j], vs[i]
}

// Less returns which vector is less than the other based on
// r3.Vector.Cmp.
func (vs Vectors) Less(i, j int) bool {
	cmp := vs[i].Cmp(vs[j])
	if cmp == 0 {
		return false
	}
	return cmp < 0
}

// Data describes data associated single point within a PointCloud.
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

// PointAndData convenient utility, try not to use too often for performance reasons.
type PointAndData struct {
	P r3.Vector
	D Data
}

// Compare utility method for kdtree.
func (v PointAndData) Compare(c kdtree.Comparable, d kdtree.Dim) float64 {
	p2, ok := c.(*PointAndData)
	if !ok {
		panic("PointAndData Compare got wrong data")
	}
	v1, v2 := v.P, p2.P
	switch d {
	case 0:
		return v1.X - v2.X
	case 1:
		return v1.Y - v2.Y
	case 2:
		return v1.Z - v2.Z
	default:
		panic("illegal dimension fed to basicPoint.Compare")
	}
}

// Dims how many dimensions are in the data, in this case always 3.
func (v PointAndData) Dims() int {
	return 3
}

// Distance for kdtree.
func (v PointAndData) Distance(c kdtree.Comparable) float64 {
	p2, ok := c.(*PointAndData)
	if !ok {
		panic("PointAndData Compare got wrong data")
	}
	v1, v2 := v.P, p2.P
	return math.Sqrt(math.Pow(v2.X-v1.X, 2) + math.Pow(v2.Y-v1.Y, 2) + math.Pow(v2.Z-v1.Z, 2))
}

// TODO(bhaney): this is gross, refactor so not needed.
func getPositions(foo []PointAndData) []r3.Vector {
	v := make([]r3.Vector, len(foo))
	for idx, x := range foo {
		v[idx] = x.P
	}
	return v
}

type basicData struct {
	hasColor bool
	c        color.NRGBA

	hasValue bool
	value    int

	intensity uint16
}

// NewBasicData returns a point that is solely positionally based.
func NewBasicData() Data {
	return &basicData{}
}

// NewColoredData returns a point that has both position and color.
func NewColoredData(c color.NRGBA) Data {
	return &basicData{c: c, hasColor: true}
}

// NewValueData returns a point that has both position and a user data value.
func NewValueData(v int) Data {
	return &basicData{value: v, hasValue: true}
}

func (bp *basicData) SetColor(c color.NRGBA) Data {
	bp.c = c
	bp.hasColor = true
	return bp
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
