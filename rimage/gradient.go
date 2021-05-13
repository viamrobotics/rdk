package rimage

import (
	"fmt"
	"image"
	"math"

	"gonum.org/v1/gonum/mat"
)

// Vec2D represents the gradient of an image at a point.
// The gradient has both a magnitude and direction.
// Magnitude has values (0, infinity) and direction is [0, 2pi)
type Vec2D struct {
	magnitude float64
	direction float64
}

// VectorField2D stores all the gradient vectors of the image
// allowing one to retrieve the gradient for any given (x,y) point
type VectorField2D struct {
	width  int
	height int

	data []Vec2D
}

// Magnitude TODO
func (g Vec2D) Magnitude() float64 {
	return g.magnitude
}

// Direction TODO
func (g Vec2D) Direction() float64 {
	return g.direction
}

func (vf *VectorField2D) kxy(x, y int) int {
	return (y * vf.width) + x
}

// Width TODO
func (vf *VectorField2D) Width() int {
	return vf.width
}

// Height TODO
func (vf *VectorField2D) Height() int {
	return vf.height
}

// Get TODO
func (vf *VectorField2D) Get(p image.Point) Vec2D {
	return vf.data[vf.kxy(p.X, p.Y)]
}

// GetVec2D TODO
func (vf *VectorField2D) GetVec2D(x, y int) Vec2D {
	return vf.data[vf.kxy(x, y)]
}

// Set TODO
func (vf *VectorField2D) Set(x, y int, val Vec2D) {
	vf.data[vf.kxy(x, y)] = val
}

// MakeEmptyVectorField2D TODO
func MakeEmptyVectorField2D(width, height int) VectorField2D {
	vf := VectorField2D{
		width:  width,
		height: height,
		data:   make([]Vec2D, width*height),
	}

	return vf
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
		return nil, fmt.Errorf("cannot make VectorField2D from two matrices of different sizes (%v,%v), (%v,%v)", magW, magH, dirW, dirH)
	}
	g := make([]Vec2D, 0, dirW*dirH)
	for y := 0; y < dirH; y++ {
		for x := 0; x < dirW; x++ {
			g = append(g, Vec2D{magnitude.At(y, x), direction.At(y, x)}) // in mat.Dense, indexing is (row, column)
		}
	}
	return &VectorField2D{dirW, dirH, g}, nil
}

// ToPrettyPicture creates a picture of the direction that the gradients point to in the original image.
func (vf *VectorField2D) ToPrettyPicture() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, vf.Width(), vf.Height()))
	for x := 0; x < vf.Width(); x++ {
		for y := 0; y < vf.Height(); y++ {
			p := image.Point{x, y}
			g := vf.Get(p)
			if g.Magnitude() == 0 {
				continue
			}
			deg := g.Direction() * (180. / math.Pi)
			img.Set(x, y, NewColorFromHSV(deg, 1.0, 1.0))
		}
	}
	return img
}

// Sobel filters are used to approximate the gradient of the image intensity. One filter for each direction.
var (
	sobelX = [3][3]int{{1, 0, -1}, {2, 0, -2}, {1, 0, -1}}
	sobelY = [3][3]int{{1, 2, 1}, {0, 0, 0}, {-1, -2, -1}}
)

// SobelFilter takes in a DepthMap, approximates the gradient in the X and Y direction at every pixel
// (after shaving a pixel of every side), creates a  vector in polar form, and returns a vector field.
func SobelFilter(dm *DepthMap) VectorField2D {
	width, height := dm.Width(), dm.Height()
	// taking a gradient will remove a pixel from all sides of the image
	g := make([]Vec2D, 0, (width-2)*(height-2))
	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			// apply the Sobel Filter over a 3x3 square around each pixel
			sX, sY := 0, 0
			xRange, yRange := [3]int{-1, 0, 1}, [3]int{-1, 0, 1}
			for i, dx := range xRange {
				for j, dy := range yRange {
					d := int(dm.GetDepth(x+dx, y+dy))
					// rows are height j, columns are width i
					sX += sobelX[j][i] * d
					sY += sobelY[j][i] * d
				}
			}
			mag, dir := getMagnitudeAndDirection(float64(sX), float64(sY))
			g = append(g, Vec2D{mag, dir})
		}
	}
	vf := VectorField2D{width - 2, height - 2, g}
	return vf

}

// Input a vector in Cartesian coordinates and return the vector in polar coordinates.
func getMagnitudeAndDirection(x, y float64) (float64, float64) {
	mag := math.Sqrt(x*x + y*y)
	// transform angle so that it is between [0, 2pi) rather than [-pi, pi]
	dir := math.Atan2(y, x)
	if dir < 0. {
		dir += 2. * math.Pi
	}
	return mag, dir
}
