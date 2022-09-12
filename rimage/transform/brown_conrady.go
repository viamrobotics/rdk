package transform

// BrownConrady is a struct for some terms of a modified Brown-Conrady model of distortion.
type BrownConrady struct {
	RadialK1     float64 `json:"rk1"`
	RadialK2     float64 `json:"rk2"`
	RadialK3     float64 `json:"rk3"`
	TangentialP1 float64 `json:"tp1"`
	TangentialP2 float64 `json:"tp2"`
}

// ModelType returns the type of distortion model
func (bc *BrownConrady) ModelType() DistortionType {
	return BrownConradyDistortionType
}

// Transform distorts the input points x,y according to a modified Brown-Conrady model as described by OpenCV
// https://docs.opencv.org/3.4/da/d54/group__imgproc__transform.html#ga7dfb72c9cf9780a347fbe3d1c47e5d5a
func (bc *BrownConrady) Transform(x, y float64) (float64, float64) {
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
