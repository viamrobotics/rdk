package transform

import (
	"math"

	"github.com/go-errors/errors"
	"github.com/golang/geo/r2"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/core/rimage"
)

// DepthColorHomography stores the color camera intrinsics and the depth->color homography that aligns a depth map
// with the color image. These parameters can take the color and depth image and create a point cloud of 3D points
// where the origin is the origin of the color camera, with units of mm.
type DepthColorHomography struct {
	ColorCamera  PinholeCameraIntrinsics `json:"color"`
	DepthToColor Homography              `json:"homography"`
	RotateDepth  int                     `json:"rotate"`
}

// Homography is a 3x3 matrix (represented as a 2D array) used to transform a plane from the perspective of a 2D
// camera to the perspective of another 2D camera. Indices are [row][column]
type Homography struct {
	matrix *mat.Dense
}

// NewHomography creates a Homography from a slice of floats
func NewHomography(vals []float64) *Homography {
	// add check for length of 9
	// add check for mathematical property of homography
	d := mat.NewDense(3, 3, vals)
	return &Homography{d}
}

// At returns the value of the homography at the given index
func (h *Homography) At(row, col int) float64 {
	return h.matrix.At(row, col)
}

// Apply will transform the given point according to the homography
func (h *Homography) Apply(pt r2.Point) r2.Point {
	x := h.At(0, 0)*pt.X + h.At(0, 1)*pt.Y + h.At(0, 2)
	y := h.At(1, 0)*pt.X + h.At(1, 1)*pt.Y + h.At(1, 2)
	z := h.At(2, 0)*pt.X + h.At(2, 1)*pt.Y + h.At(2, 2)
	return r2.Point{X: x / z, Y: y / z}
}

// Inverse inverts the homography. If it pointed from color -> depth, Inverse makes it point
// from depth -> color.
func (h *Homography) Inverse() (*Homography, error) {
	toMat := mat.NewDense(3, 3, []float64{
		h.At(0, 0), h.At(0, 1), h.At(0, 2),
		h.At(1, 0), h.At(1, 1), h.At(1, 2),
		h.At(2, 0), h.At(2, 1), h.At(2, 2),
	})

	var hInv mat.Dense
	err := hInv.Inverse(toMat)
	if err != nil {
		return nil, errors.Errorf("homography is not invertible (but homographies should always be invertible?): %w", err)
	}
	return &Homography{&hInv}, nil
}

// BilinearInterpolationDepth approximates the Depth value between pixels according to a bilinear
// interpolation. A nil return value means the interpolation is out of bounds.
func BilinearInterpolationDepth(pt r2.Point, dm *rimage.DepthMap) *rimage.Depth {
	width, height := dm.Width(), dm.Height()
	xmin := int(math.Floor(pt.X))
	xmax := int(math.Ceil(pt.X))
	ymin := int(math.Floor(pt.Y))
	ymax := int(math.Ceil(pt.Y))
	if xmin < 0 || ymin < 0 || xmax >= width || ymax >= height { // point out of bounds - skip it
		return nil
	}
	// get depth values
	d00 := float64(dm.GetDepth(xmin, ymin))
	d10 := float64(dm.GetDepth(xmax, ymin))
	d01 := float64(dm.GetDepth(xmin, ymax))
	d11 := float64(dm.GetDepth(xmax, ymax))
	// calculate weights
	area := float64((xmax - xmin) * (ymax - ymin))
	w00 := ((float64(xmax) - pt.X) * (float64(ymax) - pt.Y)) / area
	w10 := ((float64(xmax) - pt.X) * (pt.Y - float64(ymin))) / area
	w01 := ((pt.X - float64(xmin)) * (float64(ymax) - pt.Y)) / area
	w11 := ((pt.X - float64(xmin)) * (pt.Y - float64(ymin))) / area

	result := rimage.Depth(math.Round(d00*w00 + d01*w01 + d10*w10 + d11*w11))
	return &result
}
