// Package transform provides image transformation utilities relying on camera parameters.
//
// It also understands how to work with multiple images originating from different cameras.
package transform

import (
	"image"
	"math"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage"
)

// IntelConfig is an alignment config for some Intel camera.
var IntelConfig = AlignConfig{
	ColorInputSize:  image.Point{1280, 720},
	ColorWarpPoints: []image.Point{{0, 0}, {1196, 720}},

	DepthInputSize:  image.Point{1024, 768},
	DepthWarpPoints: []image.Point{{67, 100}, {1019, 665}},

	OutputSize: image.Point{640, 360},
}

// AlignConfig TODO.
type AlignConfig struct {
	ColorInputSize  image.Point // this validates input size
	ColorWarpPoints []image.Point

	DepthInputSize  image.Point // this validates output size
	DepthWarpPoints []image.Point

	WarpFromCommon bool
	OutputSize     image.Point
	OutputOrigin   image.Point
}

// ComputeWarpFromCommon TODO.
func (config AlignConfig) ComputeWarpFromCommon(logger logging.Logger) (*AlignConfig, error) {
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
		ColorWarpPoints: rimage.ArrayToPoints(colorPoints),
		DepthInputSize:  config.DepthInputSize,
		DepthWarpPoints: rimage.ArrayToPoints(depthPoints),
		OutputSize:      config.OutputSize,
		OutputOrigin:    config.OutputOrigin,
	}, nil
}

// CheckValid TODO.
func (config AlignConfig) CheckValid() error {
	if config.ColorInputSize.X == 0 ||
		config.ColorInputSize.Y == 0 {
		return errors.Errorf("invalid ColorInputSize %#v", config.ColorInputSize)
	}

	if config.DepthInputSize.X == 0 ||
		config.DepthInputSize.Y == 0 {
		return errors.Errorf("invalid DepthInputSize %#v", config.DepthInputSize)
	}

	if config.OutputSize.X == 0 || config.OutputSize.Y == 0 {
		return errors.Errorf("invalid OutputSize %v", config.OutputSize)
	}

	if len(config.ColorWarpPoints) != 2 && len(config.ColorWarpPoints) != 4 {
		return errors.Errorf("invalid ColorWarpPoints, has to be 2 or 4 is %d", len(config.ColorWarpPoints))
	}

	if len(config.DepthWarpPoints) != 2 && len(config.DepthWarpPoints) != 4 {
		return errors.Errorf("invalid DepthWarpPoints, has to be 2 or 4 is %d", len(config.DepthWarpPoints))
	}

	return nil
}

// ImageAlign returns points suitable for calling warp on.
func ImageAlign(
	img1Size image.Point,
	img1Points []image.Point,
	img2Size image.Point,
	img2Points []image.Point,
	logger logging.Logger,
) ([]image.Point, []image.Point, error) {
	if len(img1Points) != 2 || len(img2Points) != 2 {
		return nil, nil, errors.New("need exactly 2 matching points")
	}

	fixPoints := func(pts []image.Point) []image.Point {
		r := rimage.BoundingBox(pts)
		return rimage.ArrayToPoints([]image.Point{r.Min, r.Max})
	}

	// this only works for things on a multiple of 90 degrees apart, not arbitrary

	// firse we figure out if we are rotated 90 degrees or not to know which direction to expand
	colorAngle := rimage.PointAngle(img1Points[0], img1Points[1])
	depthAngle := rimage.PointAngle(img2Points[0], img2Points[1])

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

	if true {
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
		return nil, nil, err
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
		return nil, nil, err
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
		return nil, nil, err
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
		return nil, nil, err
	}
	// Set the crop coorindates for the images
	newImg1Points := make([]image.Point, len(img1Points))
	copy(newImg1Points, img1Points)
	newImg2Points := make([]image.Point, len(img2Points))
	copy(newImg2Points, img2Points)
	newImg1Points[0].X, newImg1Points[0].Y = trimLeft*trimFirstLeft, trimTop*trimFirstTop
	newImg1Points[1].X, newImg1Points[1].Y = (img1Size.X-1)-trimRight*trimFirstRight, (img1Size.Y-1)-trimBot*trimFirstBot
	if rotated {
		newImg2Points[0].X, newImg2Points[0].Y = trimBot*(1-trimFirstBot), trimLeft*(1-trimFirstLeft)
		newImg2Points[1].X, newImg2Points[1].Y = (img2Size.X-1)-trimTop*(1-trimFirstTop), (img2Size.Y-1)-trimRight*(1-trimFirstRight)
	} else {
		newImg2Points[0].X, newImg2Points[0].Y = trimLeft*(1-trimFirstLeft), trimTop*(1-trimFirstTop)
		newImg2Points[1].X, newImg2Points[1].Y = (img2Size.X-1)-trimRight*(1-trimFirstRight), (img2Size.Y-1)-trimBot*(1-trimFirstBot)
	}

	if true {
		logger.Debugf("img1 size: %v img1 points: %v", img1Size, newImg1Points)
		logger.Debugf("img2 size: %v img2 points: %v", img2Size, newImg2Points)
		if !rimage.AllPointsIn(img1Size, newImg1Points) || !rimage.AllPointsIn(img2Size, newImg2Points) {
			logger.Debugf(
				"Points are not contained in the images: %v %v",
				rimage.AllPointsIn(img1Size, newImg1Points),
				rimage.AllPointsIn(img2Size, newImg2Points),
			)
		}
	}
	newImg1Points = fixPoints(newImg1Points)
	newImg2Points = fixPoints(newImg2Points)

	if rotated {
		// TODO(erh): handle flipped
		newImg2Points = rotatePoints(newImg2Points)
	}

	return newImg1Points, newImg2Points, nil
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
		return -1, -1, errors.Errorf(
			"both img1Pt1Dist (%v) and img2Pt1Dist (%v) must be greater than (or both less than)"+
				"their respective img1Pt2Dist (%v) and img2Pt2Dist (%v)",
			img1Pt1Dist,
			img2Pt1Dist,
			img1Pt2Dist,
			img2Pt2Dist,
		)
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

	return -1, -1, errors.Errorf("ratios were not comparable ratioA: %v, ratio1: %v", ratioA, ratio1)
}
