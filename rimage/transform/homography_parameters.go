package transform

import (
	"fmt"

	"github.com/golang/geo/r2"
)

// DepthColorHomography stores the color camera intrinsics and the depth->color homography that aligns a depth map
// with the color image. These parameters can take the color and depth image and create a point cloud of 3D points
// where the origin is the origin of the color camera, with units of mm.
type DepthColorHomography struct {
	ColorCamera PinholeCameraIntrinsics `json:"color"`
	DepthCamera Homography              `json:"depth"`
	RotateDepth int                     `json:"rotate"`
}

// Homography is a 3x3 matrix (represented as a 2D array) used to transform a plane from the perspective of a 2D
// camera to the perspective of another 2D camera. Indices are [row][column]
type Homography [3][3]float64

func (h *Homography) At(row, col int) float64 {
	return h[row][col]
}

func (h *Homography) Apply(pt r2.Point) []r2.Point {
	x := h.At(0, 0)*pt.X + h.At(0, 1)*pt.Y + h.At(0, 2)
	y := h.At(1, 0)*pt.X + h.At(1, 1)*pt.Y + h.At(1, 2)
	z := h.At(2, 0)*pt.X + h.At(2, 1)*pt.Y + h.At(2, 2)
	return r2.Point{X: x / z, Y: y / z}
}

func BicubicInterpolation() {
}
