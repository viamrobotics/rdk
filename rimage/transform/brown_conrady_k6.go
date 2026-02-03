package transform

import "github.com/pkg/errors"

// BrownConradyK6 is a struct for the terms of the Brown-Conrady model of distortion with 6 radial coefficients.
type BrownConradyK6 struct {
	RadialK1     float64 `json:"rk1"`
	RadialK2     float64 `json:"rk2"`
	RadialK3     float64 `json:"rk3"`
	RadialK4     float64 `json:"rk4"`
	RadialK5     float64 `json:"rk5"`
	RadialK6     float64 `json:"rk6"`
	TangentialP1 float64 `json:"tp1"`
	TangentialP2 float64 `json:"tp2"`
}

// CheckValid checks if the BrownConradyK6 shaped distortion_parameters are valid.
func (bc *BrownConradyK6) CheckValid() error {
	if bc == nil {
		return InvalidDistortionError("BrownConradyK6 shaped distortion_parameters not provided")
	}
	return nil
}

// NewBrownConradyK6 returns a new BrownConradyK6 from a list of parameters.
// The parameters are expected to be in the order: [rk1, rk2, rk3, rk4, rk5, rk6, tp1, tp2]
func NewBrownConradyK6(inp []float64) (*BrownConradyK6, error) {
	if len(inp) > 8 {
		return nil, errors.Errorf("list of parameters too long, expected max 8, got %d", len(inp))
	}
	if len(inp) == 0 {
		return &BrownConradyK6{}, nil
	}
	// Deep copy the input slice
	inp_copy := make([]float64, len(inp))
	copy(inp_copy, inp)
	// Fill missing values with 0.0
	for i := len(inp); i < 8; i++ {
		inp_copy = append(inp_copy, 0.0)
	}
	return &BrownConradyK6{inp_copy[0], inp_copy[1], inp_copy[2], inp_copy[3], inp_copy[4], inp_copy[5], inp_copy[6], inp_copy[7]}, nil
}

// ModelType returns the distortion model type.
func (bc *BrownConradyK6) ModelType() DistortionType {
	return BrownConradyK6DistortionType
}

// Parameters returns the distortion parameters of the Brown-Conrady K6 model as a list of floats.
func (bc *BrownConradyK6) Parameters() []float64 {
	if bc == nil {
		return []float64{}
	}
	return []float64{bc.RadialK1, bc.RadialK2, bc.RadialK3, bc.RadialK4, bc.RadialK5, bc.RadialK6, bc.TangentialP1, bc.TangentialP2}
}

// Transform distorts the input points x,y according to the Brown-Conrady K6 model as described by OpenCV
// https://docs.opencv.org/3.4/da/d54/group__imgproc__transform.html#ga7dfb72c9cf9780a347fbe3d1c47e5d5a
func (bc *BrownConradyK6) Transform(x, y float64) (float64, float64) {
	if bc == nil {
		return x, y
	}
	r2 := x*x + y*y
	// Horner's method for radial distortion polynomial: 1 + k1*r^2 + k2*r^4 + k3*r^6 + k4*r^8 + k5*r^10 + k6*r^12
	radDist := 1. + r2*(bc.RadialK1+r2*(bc.RadialK2+r2*(bc.RadialK3+r2*(bc.RadialK4+r2*(bc.RadialK5+r2*bc.RadialK6)))))
	radDistX := x * radDist
	radDistY := y * radDist
	tanDistX := 2.*bc.TangentialP1*x*y + bc.TangentialP2*(r2+2.*x*x)
	tanDistY := 2.*bc.TangentialP2*x*y + bc.TangentialP1*(r2+2.*y*y)
	resX := radDistX + tanDistX
	resY := radDistY + tanDistY
	return resX, resY
}
