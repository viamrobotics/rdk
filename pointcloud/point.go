package pointcloud

import (
	"encoding"
	"image/color"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
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

	// BinaryMarshaler allows the marshaling of Data into a list of bytes.
	encoding.BinaryMarshaler

	// BinaryUnmarshaler allows the unmarshaling of a list of bytes in Data.
	encoding.BinaryUnmarshaler
}

type basicData struct {
	hasColor bool
	c        color.NRGBA

	hasValue bool
	value    int
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

// MarshalBinary checks for the presence of color and value data. If present, it will store said data in an order byte
// array.
func (bp *basicData) MarshalBinary() (data []byte, err error) {
	var dataBytes []byte

	if bp.HasColor() {
		dataBytes = append(dataBytes, bp.c.R)
		dataBytes = append(dataBytes, bp.c.G)
		dataBytes = append(dataBytes, bp.c.B)
		dataBytes = append(dataBytes, bp.c.A)
	}
	if bp.HasValue() {
		dataBytes = append(dataBytes, byte(bp.Value()))
	}
	return dataBytes, nil
}

// UnmarshalBinary takes in a bytes array and checking the length and setting the color and values appropriately.
func (bp *basicData) UnmarshalBinary(dataBytes []byte) error {
	switch len(dataBytes) {
	case 5:
		bp.SetColor(color.NRGBA{R: dataBytes[0], G: dataBytes[1], B: dataBytes[2], A: dataBytes[3]})
		bp.SetValue(int(dataBytes[4]))
	case 4:
		bp.SetColor(color.NRGBA{R: dataBytes[0], G: dataBytes[1], B: dataBytes[2], A: dataBytes[3]})
	case 1:
		bp.SetValue(int(dataBytes[0]))
	case 0:
	default:
		// Invalid data packet size
		return errors.Errorf("error unmarshaling data invalid packet size (%d)", len(dataBytes))
	}
	return nil
}
