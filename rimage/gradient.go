package rimage

import (
	"fmt"
	"image"
	"math"

	"gonum.org/v1/gonum/mat"
)

type PolarVec struct {
	magnitude float64
	direction float64
}

type VectorField2D struct {
	width  int
	height int

	data []PolarVec
}

func (g PolarVec) Magnitude() float64 {
	return g.magnitude
}

func (g PolarVec) Direction() float64 {
	return g.direction
}

func (vf *VectorField2D) kxy(x, y int) int {
	return (y * vf.width) + x
}

func (vf *VectorField2D) Width() int {
	return vf.width
}

func (vf *VectorField2D) Height() int {
	return vf.height
}

func (vf *VectorField2D) Get(p image.Point) PolarVec {
	return vf.data[vf.kxy(p.X, p.Y)]
}

func (vf *VectorField2D) GetPolarVec(x, y int) PolarVec {
	return vf.data[vf.kxy(x, y)]
}

func (vf *VectorField2D) Set(x, y int, val PolarVec) {
	vf.data[vf.kxy(x, y)] = val
}

func MakeEmptyVectorField2D(width, height int) VectorField2D {
	vf := VectorField2D{
		width:  width,
		height: height,
		data:   make([]PolarVec, width*height),
	}

	return vf
}

func (vf *VectorField2D) MagnitudeField() *mat.Dense {
	h, w := vf.Height(), vf.Width()
	mag := make([]float64, 0, h*w)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			mag = append(mag, vf.GetPolarVec(x, y).Magnitude())
		}
	}
	return mat.NewDense(h, w, mag)
}

func (vf *VectorField2D) DirectionField() *mat.Dense {
	h, w := vf.Height(), vf.Width()
	dir := make([]float64, 0, h*w)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dir = append(dir, vf.GetPolarVec(x, y).Direction())
		}
	}
	return mat.NewDense(h, w, dir)
}

func MakeVectorField2DFromDense(magnitude, direction *mat.Dense) (*VectorField2D, error) {
	magH, magW := magnitude.Dims()
	dirH, dirW := direction.Dims()
	if magW != dirW && magH != dirH {
		return nil, fmt.Errorf("cannot make VectorField2D from two matrices of different sizes (%v,%v), (%v,%v)", magW, magH, dirW, dirH)
	}
	g := make([]PolarVec, 0, dirW*dirH)
	for y := 0; y < dirH; y++ {
		for x := 0; x < dirW; x++ {
			g = append(g, PolarVec{magnitude.At(y, x), direction.At(y, x)}) // in mat.Dense, indexing is (row, column)
		}
	}
	return &VectorField2D{dirW, dirH, g}, nil
}

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

// Sobel Filter takes in a DepthMap, approximates the gradient in the X and Y direction at every pixel
// (after shaving a pixel of every side), creates a  vector in polar form, and returns a vector field.
func SobelFilter(dm *DepthMap) VectorField2D {
	width, height := dm.Width(), dm.Height()
	// taking a gradient will remove a pixel from all sides of the image
	g := make([]PolarVec, 0, (width-2)*(height-2))
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
			g = append(g, PolarVec{mag, dir})
		}
	}
	vf := VectorField2D{width - 2, height - 2, g}
	return vf

}

// Input a vector in cartesian coordinates and return the vector in polar coordinates.
func getMagnitudeAndDirection(x, y float64) (float64, float64) {
	mag := math.Sqrt(x*x + y*y)
	// transform angle so that it is between [0, 2pi) rather than [-pi, pi]
	dir := math.Atan2(y, x)
	if dir < 0. {
		dir += 2. * math.Pi
	}
	return mag, dir
}
