package transform

import (
	"github.com/golang/geo/r2"
	"github.com/pkg/errors"
	"gonum.org/v1/gonum/mat"
)

// RawPinholeCameraHomography is a structure that can be easily serialized and unserialized into JSON.
type RawPinholeCameraHomography struct {
	ColorCamera  PinholeCameraIntrinsics `json:"color"`
	Homography   []float64               `json:"transform"`
	DepthToColor bool                    `json:"depth_to_color"`
	RotateDepth  int                     `json:"rotate_depth"`
}

// CheckValid runs checks on the fields of the struct to see if the inputs are valid.
func (rdch *RawPinholeCameraHomography) CheckValid() error {
	if rdch == nil {
		return errors.New("pointer to PinholeCameraHomography is nil")
	}
	if rdch.Homography == nil {
		return errors.New("pointer to Homography is nil")
	}
	if rdch.ColorCamera.Width == 0 || rdch.ColorCamera.Height == 0 {
		return errors.Errorf("invalid ColorSize (%#v, %#v)", rdch.ColorCamera.Width, rdch.ColorCamera.Height)
	}
	if len(rdch.Homography) != 9 {
		return errors.Errorf("input to NewHomography must have length of 9. Has length of %d", len(rdch.Homography))
	}
	return nil
}

// Homography is a 3x3 matrix used to transform a plane from the perspective of a 2D
// camera to the perspective of another 2D camera.
type Homography struct {
	matrix *mat.Dense
}

// NewHomography creates a Homography from a slice of floats.
func NewHomography(vals []float64) (*Homography, error) {
	if len(vals) != 9 {
		return nil, errors.Errorf("input to NewHomography must have length of 9. Has length of %d", len(vals))
	}
	// TODO(bij): add check for mathematical property of homography
	d := mat.NewDense(3, 3, vals)
	return &Homography{d}, nil
}

// At returns the value of the homography at the given index.
func (h *Homography) At(row, col int) float64 {
	return h.matrix.At(row, col)
}

// Apply will transform the given point according to the homography.
func (h *Homography) Apply(pt r2.Point) r2.Point {
	x := h.At(0, 0)*pt.X + h.At(0, 1)*pt.Y + h.At(0, 2)
	y := h.At(1, 0)*pt.X + h.At(1, 1)*pt.Y + h.At(1, 2)
	z := h.At(2, 0)*pt.X + h.At(2, 1)*pt.Y + h.At(2, 2)
	return r2.Point{X: x / z, Y: y / z}
}

// Inverse inverts the homography. If homography went from color -> depth, Inverse makes it point
// from depth -> color.
func (h *Homography) Inverse() (*Homography, error) {
	var hInv mat.Dense
	if err := hInv.Inverse(h.matrix); err != nil {
		return nil, errors.Wrap(err, "homography is not invertible (but homographies should always be invertible?)")
	}
	return &Homography{&hInv}, nil
}
