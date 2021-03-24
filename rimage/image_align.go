package rimage

import (
	"fmt"
	"image"
	"math"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
)

type DistortionModel struct {
	RadialK1     float64 `json:"rk1"`
	RadialK2     float64 `json:"rk2"`
	RadialK3     float64 `json:"rk3"`
	TangentialP1 float64 `json:"tp1"`
	TangentialP2 float64 `json:"tp2"`
}

type PinholeCameraIntrinsics struct {
	Width      int             `json:"width"`
	Height     int             `json:"height"`
	Fx         float64         `json:"fx"`
	Fy         float64         `json:"fy"`
	Ppx        float64         `json:"ppx"`
	Ppy        float64         `json:"ppy"`
	Distortion DistortionModel `json:"distortion"`
}

type Extrinsics struct {
	RotationMatrix    []float64 `json:"rotation"`
	TranslationVector []float64 `json:"translation"`
}

type DepthColorIntrinsicsExtrinsics struct {
	ColorCamera  PinholeCameraIntrinsics `json:"color"`
	DepthCamera  PinholeCameraIntrinsics `json:"depth"`
	ExtrinsicD2C Extrinsics              `json:"extrinsicsDepthToColor"`
}

func NewDepthColorIntrinsicsExtrinsics() *DepthColorIntrinsicsExtrinsics {
	return &DepthColorIntrinsicsExtrinsics{
		ColorCamera:  PinholeCameraIntrinsics{0, 0, 0, 0, 0, 0, DistortionModel{0, 0, 0, 0, 0}},
		DepthCamera:  PinholeCameraIntrinsics{0, 0, 0, 0, 0, 0, DistortionModel{0, 0, 0, 0, 0}},
		ExtrinsicD2C: Extrinsics{[]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}, []float64{0, 0, 0}},
	}
}

// Function to transform a pixel with depth to a 3D point cloud
// the intrinsics parameters should be the ones of the sensor used to obtain the image that contains the pixel
func (params *PinholeCameraIntrinsics) PixelToPoint(x, y int, z float64) (float64, float64, float64) {
	//TODO(louise): add unit test
	xOverZ := (params.Ppx - float64(x)) / params.Fx
	yOverZ := (params.Ppy - float64(y)) / params.Fy
	// get x and y
	xPixel := xOverZ * z
	yPixel := yOverZ * z
	return xPixel, yPixel, z
}

// Function to project a 3D point to a pixel in an image plane
// the intrinsics parameters should be the ones of the sensor we want to project to
func (params *PinholeCameraIntrinsics) PointToPixel(x, y, z float64) (float64, float64) {
	//TODO(louise): add unit test
	if z != 0. {
		xPx := math.Round(x*params.Fx/(z) + params.Ppx)
		yPx := math.Round(y*params.Fy/(z) + params.Ppy)
		return xPx, yPx
	}
	// if depth is zero at this pixel, return negative coordinates so that the cropping to RGB bounds will filter it out
	return -1.0, -1.0
}

// Function to apply a rigid body transform between two cameras to a 3D point
func (params *Extrinsics) TransformPointToPoint(x, y, z float64) r3.Vector {
	rotationMatrix := params.RotationMatrix
	translationVector := params.TranslationVector
	n := len(rotationMatrix)
	if n != 9 {
		panic("Rotation Matrix to transform point cloud should be a 3x3 matrix")
	}
	xTransformed := rotationMatrix[0]*x + rotationMatrix[1]*y + rotationMatrix[2]*z + translationVector[0]
	yTransformed := rotationMatrix[3]*x + rotationMatrix[4]*y + rotationMatrix[5]*z + translationVector[1]
	zTransformed := rotationMatrix[6]*x + rotationMatrix[7]*y + rotationMatrix[8]*z + translationVector[2]

	return r3.Vector{xTransformed, yTransformed, zTransformed}
}

type AlignConfig struct {
	ColorInputSize  image.Point // this validates input size
	ColorWarpPoints []image.Point

	DepthInputSize  image.Point // this validates output size
	DepthWarpPoints []image.Point

	WarpFromCommon bool

	OutputSize image.Point
}

func (config AlignConfig) ComputeWarpFromCommon(logger golog.Logger) (*AlignConfig, error) {

	colorPoints, depthPoints, err := ImageAlign(
		config.ColorInputSize,
		config.ColorWarpPoints,
		config.DepthInputSize,
		config.DepthWarpPoints,
		logger,
	)

	if err != nil {
		return nil, err
	}

	return &AlignConfig{
		ColorInputSize:  config.ColorInputSize,
		ColorWarpPoints: ArrayToPoints(colorPoints),
		DepthInputSize:  config.DepthInputSize,
		DepthWarpPoints: ArrayToPoints(depthPoints),
		OutputSize:      config.OutputSize,
	}, nil
}

func (config AlignConfig) CheckValid() error {
	if config.ColorInputSize.X == 0 ||
		config.ColorInputSize.Y == 0 {
		return fmt.Errorf("invalid ColorInputSize %#v", config.ColorInputSize)
	}

	if config.DepthInputSize.X == 0 ||
		config.DepthInputSize.Y == 0 {
		return fmt.Errorf("invalid DepthInputSize %#v", config.DepthInputSize)
	}

	if config.OutputSize.X == 0 || config.OutputSize.Y == 0 {
		return fmt.Errorf("invalid OutputSize %v", config.OutputSize)
	}

	if len(config.ColorWarpPoints) != 2 && len(config.ColorWarpPoints) != 4 {
		return fmt.Errorf("invalid ColorWarpPoints, has to be 2 or 4 is %d", len(config.ColorWarpPoints))
	}

	if len(config.DepthWarpPoints) != 2 && len(config.DepthWarpPoints) != 4 {
		return fmt.Errorf("invalid DepthWarpPoints, has to be 2 or 4 is %d", len(config.DepthWarpPoints))
	}

	return nil
}

type DepthColorTransforms struct {
	ColorTransform, DepthTransform TransformationMatrix
	*AlignConfig                   // anonymous fields
}

func (dct *DepthColorTransforms) AlignColorAndDepth(ii *ImageWithDepth, logger golog.Logger) (*ImageWithDepth, error) {

	if ii.Color.Width() != dct.ColorInputSize.X ||
		ii.Color.Height() != dct.ColorInputSize.Y ||
		ii.Depth.Width() != dct.DepthInputSize.X ||
		ii.Depth.Height() != dct.DepthInputSize.Y {
		return nil, fmt.Errorf("unexpected aligned dimensions c:(%d,%d) d:(%d,%d) config: %#v",
			ii.Color.Width(), ii.Color.Height(), ii.Depth.Width(), ii.Depth.Height(), dct.AlignConfig)
	}
	ii.Depth.Smooth() // TODO(erh): maybe instead of this I should change warp to let the user determine how to average

	c2 := WarpImage(ii, dct.ColorTransform, dct.OutputSize)
	dm2 := ii.Depth.Warp(dct.DepthTransform, dct.OutputSize)

	return &ImageWithDepth{c2, &dm2}, nil
}

func ArrayToPoints(pts []image.Point) []image.Point {
	if len(pts) == 4 {
		return pts
	}

	if len(pts) == 2 {
		r := image.Rectangle{pts[0], pts[1]}
		return []image.Point{
			r.Min,
			{r.Max.X, r.Min.Y},
			r.Max,
			{r.Min.X, r.Max.Y},
		}
	}

	panic(fmt.Errorf("invalid number of points passed to ArrayToPoints %d", len(pts)))
}

// returns points suitable for calling warp on
func ImageAlign(img1Size image.Point, img1Points []image.Point,
	img2Size image.Point, img2Points []image.Point, logger golog.Logger) ([]image.Point, []image.Point, error) {

	debug := true

	if len(img1Points) != 2 || len(img2Points) != 2 {
		return nil, nil, fmt.Errorf("need exactly 2 matching points")
	}

	fixPoints := func(pts []image.Point) []image.Point {
		r := BoundingBox(pts)
		return ArrayToPoints([]image.Point{r.Min, r.Max})
	}

	// this only works for things on a multiple of 90 degrees apart, not arbitrary

	// firse we figure out if we are rotated 90 degrees or not to know which direction to expand
	colorAngle := PointAngle(img1Points[0], img1Points[1])
	depthAngle := PointAngle(img2Points[0], img2Points[1])

	if colorAngle < 0 {
		colorAngle += math.Pi
	}
	if depthAngle < 0 {
		depthAngle += math.Pi
	}

	colorAngle /= (math.Pi / 2)
	depthAngle /= (math.Pi / 2)

	rotated := false
	if colorAngle < 1 && depthAngle > 1 || colorAngle > 1 && depthAngle < 1 {
		rotated = true
	}

	if debug {
		logger.Debugf("colorAngle: %v depthAngle: %v rotated: %v", colorAngle, depthAngle, rotated)
	}
	// crop the four sides of the images so they enclose the same area
	// if one image is rotated, it's assumed it's the second image.
	// dist A/1 must be longer than dist B/2

	var dist1, dist2 int
	// trim top (rotated 90: trim from right)
	distA, distB := img1Points[0].Y, img1Points[1].Y
	if rotated {
		dist1, dist2 = (img2Size.X-1)-img2Points[0].X, (img2Size.X-1)-img2Points[1].X
	} else {
		dist1, dist2 = img2Points[0].Y, img2Points[1].Y
	}
	trimTop, trimFirstTop, err := trim(distA, distB, dist1, dist2)
	if err != nil {
		logger.Debugf("image_align error: %s", err)
	}
	// trim bottom (rotated 90: trim from left)
	distA, distB = (img1Size.Y-1)-img1Points[1].Y, (img1Size.Y-1)-img1Points[0].Y
	if rotated {
		dist1, dist2 = img2Points[1].X, img2Points[0].X
	} else {
		dist1, dist2 = (img2Size.Y-1)-img2Points[1].Y, (img2Size.Y-1)-img2Points[0].Y
	}
	trimBot, trimFirstBot, err := trim(distA, distB, dist1, dist2)
	if err != nil {
		logger.Debugf("image_align error: %s", err)
	}
	// trim left (rotated 90: trim from top)
	distA, distB = img1Points[1].X, img1Points[0].X
	if rotated {
		dist1, dist2 = img2Points[1].Y, img2Points[0].Y
	} else {
		dist1, dist2 = img2Points[1].X, img2Points[0].X
	}
	trimLeft, trimFirstLeft, err := trim(distA, distB, dist1, dist2)
	if err != nil {
		logger.Debugf("image_align error: %s", err)
	}
	// trim right (rotated 90: trim from bottom)
	distA, distB = (img1Size.X-1)-img1Points[0].X, (img1Size.X-1)-img1Points[1].X
	if rotated {
		dist1, dist2 = (img2Size.Y-1)-img2Points[0].Y, (img2Size.Y-1)-img2Points[1].Y
	} else {
		dist1, dist2 = (img2Size.X-1)-img2Points[0].X, (img2Size.X-1)-img2Points[1].X
	}
	trimRight, trimFirstRight, err := trim(distA, distB, dist1, dist2)
	if err != nil {
		logger.Debugf("error: %s", err)
	}
	// Set the crop coorindates for the images
	img1Points[0].X, img1Points[0].Y = trimLeft*trimFirstLeft, trimTop*trimFirstTop
	img1Points[1].X, img1Points[1].Y = (img1Size.X-1)-trimRight*trimFirstRight, (img1Size.Y-1)-trimBot*trimFirstBot
	if rotated {
		img2Points[0].X, img2Points[0].Y = trimBot*(1-trimFirstBot), trimLeft*(1-trimFirstLeft)
		img2Points[1].X, img2Points[1].Y = (img2Size.X-1)-trimTop*(1-trimFirstTop), (img2Size.Y-1)-trimRight*(1-trimFirstRight)
	} else {
		img2Points[0].X, img2Points[0].Y = trimLeft*(1-trimFirstLeft), trimTop*(1-trimFirstTop)
		img2Points[1].X, img2Points[1].Y = (img2Size.X-1)-trimRight*(1-trimFirstRight), (img2Size.Y-1)-trimBot*(1-trimFirstBot)
	}

	if debug {
		logger.Debugf("img1 size: %v img1 points: %v", img1Size, img1Points)
		logger.Debugf("img2 size: %v img2 points: %v", img2Size, img2Points)
		if !AllPointsIn(img1Size, img1Points) || !AllPointsIn(img2Size, img2Points) {
			logger.Debugf("Points are not contained in the images: %v %v", AllPointsIn(img1Size, img1Points), AllPointsIn(img2Size, img2Points))
		}
	}
	img1Points = fixPoints(img1Points)
	img2Points = fixPoints(img2Points)

	if rotated {
		// TODO(erh): handle flipped
		img2Points = rotatePoints(img2Points)
	}

	return img1Points, img2Points, nil
}

func rotatePoints(pts []image.Point) []image.Point {
	pts = append(pts[1:], pts[0])
	return pts
}

// For two images, given the distances from the image edge to two points on the image,
// trim calculates how much to trim off of one image edge to make the ratios of the distance
// from the points to the edge equal between the two images.
func trim(img1Pt1Dist, img1Pt2Dist, img2Pt1Dist, img2Pt2Dist int) (int, int, error) {

	var distA, distB, dist1, dist2 float64
	// required: distA/dist1 must be farther from the image edge that distB/dist2 so that the ratio is always > 1
	switch {
	case (img1Pt1Dist > img1Pt2Dist) && (img2Pt1Dist > img2Pt2Dist):
		distA, distB = float64(img1Pt1Dist), float64(img1Pt2Dist)
		dist1, dist2 = float64(img2Pt1Dist), float64(img2Pt2Dist)
	case (img1Pt1Dist < img1Pt2Dist) && (img2Pt1Dist < img2Pt2Dist):
		distA, distB = float64(img1Pt2Dist), float64(img1Pt1Dist)
		dist1, dist2 = float64(img2Pt2Dist), float64(img2Pt1Dist)
	default:
		return -1, -1, fmt.Errorf("both img1Pt1Dist (%v) and img2Pt1Dist (%v) must be greater than (or both less than) their respective img1Pt2Dist (%v) and img2Pt2Dist (%v)", img1Pt1Dist, img2Pt1Dist, img1Pt2Dist, img2Pt2Dist)
	}
	// returns whether to trim the first or second image, and by how much.
	var trimFirst int // 0 means trim 2nd image, 1 means trim first image
	var trimAmount float64
	ratioA := distA / distB
	ratio1 := dist1 / dist2
	// are the ratios equal already?
	const EqualityThreshold = 1e-6
	if math.Abs(ratioA-ratio1) <= EqualityThreshold {
		return int(trimAmount), trimFirst, nil
	}
	// the bigger ratio should be matched to.
	if ratioA > ratio1 {
		trimFirst = 0
		trimAmount = (distA*dist2 - distB*dist1) / (distA - distB)
		return int(math.Round(trimAmount)), trimFirst, nil
	}
	if ratioA < ratio1 {
		trimFirst = 1
		trimAmount = (dist1*distB - dist2*distA) / (dist1 - dist2)
		return int(math.Round(trimAmount)), trimFirst, nil
	}

	return -1, -1, fmt.Errorf("ratios were not comparable ratioA: %v, ratio1: %v", ratioA, ratio1)

}
