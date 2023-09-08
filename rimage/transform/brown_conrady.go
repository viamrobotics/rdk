//go:build cgo
package transform

import "github.com/pkg/errors"

// BrownConrady is a struct for some terms of a modified Brown-Conrady model of distortion.
type BrownConrady struct {
	RadialK1     float64 `json:"rk1"`
	RadialK2     float64 `json:"rk2"`
	RadialK3     float64 `json:"rk3"`
	TangentialP1 float64 `json:"tp1"`
	TangentialP2 float64 `json:"tp2"`
}

// CheckValid checks if the fields for BrownConrady have valid inputs.
func (bc *BrownConrady) CheckValid() error {
	if bc == nil {
		return InvalidDistortionError("BrownConrady shaped distortion_parameters not provided")
	}
	return nil
}

// NewBrownConrady takes in a slice of floats that will be passed into the struct in order.
func NewBrownConrady(inp []float64) (*BrownConrady, error) {
	if len(inp) > 5 {
		return nil, errors.Errorf("list of parameters too long, expected max 5, got %d", len(inp))
	}
	if len(inp) == 0 {
		return &BrownConrady{}, nil
	}
	for i := len(inp); i < 5; i++ { // fill missing values with 0.0
		inp = append(inp, 0.0)
	}
	return &BrownConrady{inp[0], inp[1], inp[2], inp[3], inp[4]}, nil
}

// ModelType returns the type of distortion model.
func (bc *BrownConrady) ModelType() DistortionType {
	return BrownConradyDistortionType
}

// Parameters returns the parameters of the distortion model as a list of floats.
func (bc *BrownConrady) Parameters() []float64 {
	if bc == nil {
		return []float64{}
	}
	return []float64{bc.RadialK1, bc.RadialK2, bc.RadialK3, bc.TangentialP1, bc.TangentialP2}
}

// Transform distorts the input points x,y according to a modified Brown-Conrady model as described by OpenCV
// https://docs.opencv.org/3.4/da/d54/group__imgproc__transform.html#ga7dfb72c9cf9780a347fbe3d1c47e5d5a
func (bc *BrownConrady) Transform(x, y float64) (float64, float64) {
	if bc == nil {
		return x, y
	}
	r2 := x*x + y*y
	radDist := (1. + bc.RadialK1*r2 + bc.RadialK2*r2*r2 + bc.RadialK3*r2*r2*r2)
	radDistX := x * radDist
	radDistY := y * radDist
	tanDistX := 2.*bc.TangentialP1*x*y + bc.TangentialP2*(r2+2.*x*x)
	tanDistY := 2.*bc.TangentialP2*x*y + bc.TangentialP1*(r2+2.*y*y)
	resX := radDistX + tanDistX
	resY := radDistY + tanDistY
	return resX, resY
}
