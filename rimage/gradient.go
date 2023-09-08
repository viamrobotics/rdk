//go:build cgo
package rimage

import (
	"image"
	"image/color"
	"math"

	"github.com/pkg/errors"
	"gonum.org/v1/gonum/mat"
)

// Vec2D represents the gradient of an image at a point.
// The gradient has both a magnitude and direction.
// Magnitude has values (0, infinity) and direction is [-pi, pi].
type Vec2D struct {
	magnitude float64
	direction float64
}

// VectorField2D stores all the gradient vectors of the image
// allowing one to retrieve the gradient for any given (x,y) point.
type VectorField2D struct {
	width  int
	height int

	data         []Vec2D
	maxMagnitude float64
}

// Magnitude TODO.
func (g Vec2D) Magnitude() float64 {
	return g.magnitude
}

// Direction TODO.
func (g Vec2D) Direction() float64 {
	return g.direction
}

// Unit returns the Vec2D with magnitude 1.
func (g Vec2D) Unit() Vec2D {
	return Vec2D{1., g.direction}
}

// Cartesian returns the componnents of the vector in cartesian coordinates.
func (g Vec2D) Cartesian() (float64, float64) {
	x := g.Magnitude() * math.Cos(g.Direction())
	y := g.Magnitude() * math.Sin(g.Direction())
	return x, y
}

// NewVec2D creates a new 2D vector given a magnitude and direction.
func NewVec2D(mag, dir float64) Vec2D {
	if mag < 0. {
		panic("vector cannot have magnitude less than 0")
	}
	if dir > math.Pi || dir < -math.Pi {
		panic("vector direction can only be between -pi and pi")
	}
	return Vec2D{mag, dir}
}

func (vf *VectorField2D) kxy(x, y int) int {
	return (y * vf.width) + x
}

// MaxMagnitude returns the largest magnitude value in the field.
func (vf *VectorField2D) MaxMagnitude() float64 {
	return vf.maxMagnitude
}

// Width TODO.
func (vf *VectorField2D) Width() int {
	return vf.width
}

// Height TODO.
func (vf *VectorField2D) Height() int {
	return vf.height
}

// Contains returns whether the given point is in the vector field.
func (vf *VectorField2D) Contains(x, y int) bool {
	return x >= 0 && y >= 0 && x < vf.width && y < vf.height
}

// Get TODO.
func (vf *VectorField2D) Get(p image.Point) Vec2D {
	return vf.data[vf.kxy(p.X, p.Y)]
}

// GetVec2D TODO.
func (vf *VectorField2D) GetVec2D(x, y int) Vec2D {
	return vf.data[vf.kxy(x, y)]
}

// Set TODO.
func (vf *VectorField2D) Set(x, y int, val Vec2D) {
	vf.data[vf.kxy(x, y)] = val
	vf.maxMagnitude = math.Max(math.Abs(val.Magnitude()), vf.maxMagnitude)
}

// MakeEmptyVectorField2D TODO.
func MakeEmptyVectorField2D(width, height int) VectorField2D {
	vf := VectorField2D{
		width:        width,
		height:       height,
		data:         make([]Vec2D, width*height),
		maxMagnitude: 0.0,
	}

	return vf
}

// Blur takes in a radius and creates a new blurred vector field from the input vector field.
func (vf *VectorField2D) Blur(radius int) VectorField2D {
	k := 1 + 2*radius
	newVF := MakeEmptyVectorField2D(vf.Width(), vf.Height())
	blur := vectorBlurFilter(k)
	for y := 0; y < vf.Height(); y++ {
		for x := 0; x < vf.Width(); x++ {
			point := image.Point{x, y}
			blurredVec := blur(point, vf)
			newVF.Set(x, y, blurredVec)
		}
	}
	return newVF
}

// MagnitudeField gets all the magnitudes of the gradient in the image as a mat.Dense.
func (vf *VectorField2D) MagnitudeField() *mat.Dense {
	h, w := vf.Height(), vf.Width()
	mag := make([]float64, 0, h*w)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			mag = append(mag, vf.GetVec2D(x, y).Magnitude())
		}
	}
	return mat.NewDense(h, w, mag)
}

// DirectionField gets all the directions of the gradient in the image as a mat.Dense.
func (vf *VectorField2D) DirectionField() *mat.Dense {
	h, w := vf.Height(), vf.Width()
	dir := make([]float64, 0, h*w)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dir = append(dir, vf.GetVec2D(x, y).Direction())
		}
	}
	return mat.NewDense(h, w, dir)
}

// VectorField2DFromDense returns a vector from a mat.Dense of both the magnitude and direction
// of the gradients of an image.
func VectorField2DFromDense(magnitude, direction *mat.Dense) (*VectorField2D, error) {
	magH, magW := magnitude.Dims()
	dirH, dirW := direction.Dims()
	if magW != dirW && magH != dirH {
		return nil, errors.Errorf("cannot make VectorField2D from two matrices of different sizes (%v,%v), (%v,%v)", magW, magH, dirW, dirH)
	}
	maxMag := 0.0
	g := make([]Vec2D, 0, dirW*dirH)
	for y := 0; y < dirH; y++ {
		for x := 0; x < dirW; x++ {
			g = append(g, Vec2D{magnitude.At(y, x), direction.At(y, x)}) // in mat.Dense, indexing is (row, column)
			maxMag = math.Max(math.Abs(magnitude.At(y, x)), maxMag)
		}
	}
	return &VectorField2D{dirW, dirH, g, maxMag}, nil
}

// MagnitudePicture creates a picture of the magnitude that the gradients point to in the original image.
func (vf *VectorField2D) MagnitudePicture() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, vf.Width(), vf.Height()))
	for x := 0; x < vf.Width(); x++ {
		for y := 0; y < vf.Height(); y++ {
			p := image.Point{x, y}
			g := vf.Get(p)
			val := uint8((g.Magnitude() / vf.maxMagnitude) * 255)
			img.Set(x, y, color.Gray{val})
		}
	}
	return img
}

// DirectionPicture creates a picture of the direction that the gradients point to in the original image.
func (vf *VectorField2D) DirectionPicture() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, vf.Width(), vf.Height()))
	for x := 0; x < vf.Width(); x++ {
		for y := 0; y < vf.Height(); y++ {
			p := image.Point{x, y}
			g := vf.Get(p)
			if g.Magnitude() == 0 {
				continue
			}
			deg := radZeroTo2Pi(g.Direction()) * (180. / math.Pi)
			img.Set(x, y, NewColorFromHSV(deg, 1.0, 1.0))
		}
	}
	return img
}

// changes the radians from between -pi,pi to 0,2pi.
func radZeroTo2Pi(rad float64) float64 {
	if rad < 0. {
		rad += 2. * math.Pi
	}
	return rad
}

// Input a vector in cartesian coordinates and return the vector in polar coordinates.
func getMagnitudeAndDirection(x, y float64) (float64, float64) {
	mag := math.Sqrt(x*x + y*y)
	dir := math.Atan2(y, x)
	return mag, dir
}
