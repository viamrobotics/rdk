package transform

import "github.com/pkg/errors"

// InverseBrownConrady applies the inverse of the Brown-Conrady distortion model.
// Given distorted points, it computes the corresponding undistorted points using
// an iterative Newton-Raphson method.
type InverseBrownConrady struct {
	RadialK1     float64 `json:"rk1"`
	RadialK2     float64 `json:"rk2"`
	RadialK3     float64 `json:"rk3"`
	TangentialP1 float64 `json:"tp1"`
	TangentialP2 float64 `json:"tp2"`
}

// CheckValid checks if the fields for InverseBrownConrady have valid inputs.
func (ibc *InverseBrownConrady) CheckValid() error {
	if ibc == nil {
		return InvalidDistortionError("InverseBrownConrady shaped distortion_parameters not provided")
	}
	return nil
}

// NewInverseBrownConrady takes in a slice of floats that will be passed into the struct in order.
func NewInverseBrownConrady(inp []float64) (*InverseBrownConrady, error) {
	if len(inp) > 5 {
		return nil, errors.Errorf("list of parameters too long, expected max 5, got %d", len(inp))
	}
	if len(inp) == 0 {
		return &InverseBrownConrady{}, nil
	}
	for i := len(inp); i < 5; i++ { // fill missing values with 0.0
		inp = append(inp, 0.0)
	}
	return &InverseBrownConrady{inp[0], inp[1], inp[2], inp[3], inp[4]}, nil
}

// ModelType returns the type of distortion model.
func (ibc *InverseBrownConrady) ModelType() DistortionType {
	return InverseBrownConradyDistortionType
}

// Parameters returns the parameters of the distortion model as a list of floats.
func (ibc *InverseBrownConrady) Parameters() []float64 {
	if ibc == nil {
		return []float64{}
	}
	return []float64{ibc.RadialK1, ibc.RadialK2, ibc.RadialK3, ibc.TangentialP1, ibc.TangentialP2}
}

// Transform applies the inverse Brown-Conrady distortion to convert distorted points
// to undistorted points. It uses an iterative Newton-Raphson method to find the
// undistorted coordinates that would produce the given distorted coordinates.
//
// The forward Brown-Conrady model is:
//
//	x_d = x_u * (1 + k1*r² + k2*r⁴ + k3*r⁶) + 2*p1*x_u*y_u + p2*(r² + 2*x_u²)
//	y_d = y_u * (1 + k1*r² + k2*r⁴ + k3*r⁶) + 2*p2*x_u*y_u + p1*(r² + 2*y_u²)
//
// where (x_d, y_d) are distorted coordinates and (x_u, y_u) are undistorted coordinates.
// This function solves for (x_u, y_u) given (x_d, y_d).
func (ibc *InverseBrownConrady) Transform(xd, yd float64) (float64, float64) {
	if ibc == nil {
		return xd, yd
	}

	// Start with the distorted point as initial guess
	xu, yu := xd, yd

	// Newton-Raphson iterations
	const maxIterations = 20
	const tolerance = 1e-10

	for i := 0; i < maxIterations; i++ {
		r2 := xu*xu + yu*yu
		r4 := r2 * r2
		r6 := r4 * r2

		// Compute forward distortion at current estimate
		radDist := 1.0 + ibc.RadialK1*r2 + ibc.RadialK2*r4 + ibc.RadialK3*r6
		tanDistX := 2.0*ibc.TangentialP1*xu*yu + ibc.TangentialP2*(r2+2.0*xu*xu)
		tanDistY := 2.0*ibc.TangentialP2*xu*yu + ibc.TangentialP1*(r2+2.0*yu*yu)

		xdEst := xu*radDist + tanDistX
		ydEst := yu*radDist + tanDistY

		// Compute error
		errX := xdEst - xd
		errY := ydEst - yd

		// Check for convergence
		if errX*errX+errY*errY < tolerance*tolerance {
			break
		}

		// Compute Jacobian of the forward distortion function
		// J = [[dxd/dxu, dxd/dyu], [dyd/dxu, dyd/dyu]]
		dRadDistDxu := 2.0 * xu * (ibc.RadialK1 + 2.0*ibc.RadialK2*r2 + 3.0*ibc.RadialK3*r4)
		dRadDistDyu := 2.0 * yu * (ibc.RadialK1 + 2.0*ibc.RadialK2*r2 + 3.0*ibc.RadialK3*r4)

		dxdDxu := radDist + xu*dRadDistDxu + 2.0*ibc.TangentialP1*yu + ibc.TangentialP2*(2.0*xu+4.0*xu)
		dxdDyu := xu*dRadDistDyu + 2.0*ibc.TangentialP1*xu + ibc.TangentialP2*2.0*yu
		dydDxu := yu*dRadDistDxu + 2.0*ibc.TangentialP2*yu + ibc.TangentialP1*2.0*xu
		dydDyu := radDist + yu*dRadDistDyu + 2.0*ibc.TangentialP2*xu + ibc.TangentialP1*(2.0*yu+4.0*yu)

		// Invert the 2x2 Jacobian and apply Newton-Raphson update
		det := dxdDxu*dydDyu - dxdDyu*dydDxu
		if det == 0 {
			break
		}

		// Update: [xu, yu] -= J^-1 * [errX, errY]
		xu -= (dydDyu*errX - dxdDyu*errY) / det
		yu -= (-dydDxu*errX + dxdDxu*errY) / det
	}

	return xu, yu
}
