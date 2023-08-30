package transform

import (
	"image"
	"image/color"
	"math"

	"github.com/golang/geo/r3"
	"github.com/montanaflynn/stats"
	"github.com/pkg/errors"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/spatialmath"
)

// ParallelProjection to pointclouds are done in a naive way that don't take any camera parameters into account.
// These are not great projections, and should really only be used for testing or artistic purposes.
type ParallelProjection struct{}

// RGBDToPointCloud take a 2D image with depth and project to a 3D point cloud.
func (pp *ParallelProjection) RGBDToPointCloud(
	img *rimage.Image,
	dm *rimage.DepthMap,
	crop ...image.Rectangle,
) (pointcloud.PointCloud, error) {
	if img == nil {
		return nil, errors.New("no rgb image to project to pointcloud")
	}
	if dm == nil {
		return nil, errors.New("no depth map to project to pointcloud")
	}
	if dm.Bounds() != img.Bounds() {
		return nil, errors.Errorf("rgb image and depth map are not the same size img(%v) != depth(%v)", img.Bounds(), dm.Bounds())
	}
	var rect *image.Rectangle
	if len(crop) > 1 {
		return nil, errors.Errorf("cannot have more than one cropping rectangle, got %v", crop)
	}
	if len(crop) == 1 {
		rect = &crop[0]
	}
	startX, startY := 0, 0
	endX, endY := img.Width(), img.Height()
	if rect != nil {
		newBounds := rect.Intersect(img.Bounds())
		startX, startY = newBounds.Min.X, newBounds.Min.Y
		endX, endY = newBounds.Max.X, newBounds.Max.Y
	}
	pc := pointcloud.New()
	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			z := dm.GetDepth(x, y)
			if z == 0 {
				continue
			}
			c := img.GetXY(x, y)
			r, g, b := c.RGB255()
			err := pc.Set(pointcloud.NewVector(float64(x), float64(y), float64(z)), pointcloud.NewColoredData(color.NRGBA{r, g, b, 255}))
			if err != nil {
				return nil, err
			}
		}
	}
	return pc, nil
}

// PointCloudToRGBD assumes the x,y coordinates are the same as the x,y pixels.
func (pp *ParallelProjection) PointCloudToRGBD(cloud pointcloud.PointCloud) (*rimage.Image, *rimage.DepthMap, error) {
	meta := cloud.MetaData()
	// Needs to be a pointcloud with color
	if !meta.HasColor {
		return nil, nil, errors.New("pointcloud has no color information, cannot create an image with depth")
	}
	// Image and DepthMap will be in the camera frame of the RGB camera.
	// Points outside of the frame will be discarded.
	// Assumption is that points in pointcloud are in mm.
	width := int(meta.MaxX - meta.MinX)
	height := int(meta.MaxY - meta.MinY)
	color := rimage.NewImage(width, height)
	depth := rimage.NewEmptyDepthMap(width, height)
	cloud.Iterate(0, 0, func(pt r3.Vector, data pointcloud.Data) bool {
		j := pt.X - meta.MinX
		i := pt.Y - meta.MinY
		x, y := int(math.Round(j)), int(math.Round(i))
		z := int(pt.Z)
		// if point has color and is inside the RGB image bounds, add it to the images
		if x >= 0 && x < width && y >= 0 && y < height && data != nil && data.HasColor() {
			r, g, b := data.RGB255()
			color.Set(image.Point{x, y}, rimage.NewColor(r, g, b))
			depth.Set(x, y, rimage.Depth(z))
		}
		return true
	})
	return color, depth, nil
}

// ImagePointTo3DPoint takes the 2D pixel point and assumes that it represents the X,Y coordinate in mm as well.
func (pp *ParallelProjection) ImagePointTo3DPoint(pt image.Point, d rimage.Depth) (r3.Vector, error) {
	return r3.Vector{X: float64(pt.X), Y: float64(pt.Y), Z: float64(d)}, nil
}

// ParallelProjectionOntoXYWithRobotMarker allows the creation of a 2D projection of a pointcloud and robot
// position onto the XY plane.
type ParallelProjectionOntoXYWithRobotMarker struct {
	robotPose *spatialmath.Pose
}

const (
	sigmaLevel        = 7    // level of precision for stdev calculation (determined through experimentation)
	imageHeight       = 1080 // image height
	imageWidth        = 1080 // image width
	missThreshold     = 0.49 // probability limit below which the associated point is assumed to be free space
	hitThreshold      = 0.55 // probability limit above which the associated point is assumed to indicate an obstacle
	pointRadius       = 1    // radius of pointcloud point
	robotMarkerRadius = 5    // radius of robot marker point
)

// Struct containing the mean and stdev.
type meanStdev struct {
	mean  float64
	stdev float64
}

// Calculates the mean and standard deviation of the X and Y coordinates stored in the point cloud.
func calculatePointCloudMeanAndStdevXY(cloud pointcloud.PointCloud) (meanStdev, meanStdev, error) {
	var X, Y []float64
	var x, y meanStdev

	cloud.Iterate(0, 0, func(pt r3.Vector, data pointcloud.Data) bool {
		X = append(X, pt.X)
		Y = append(Y, pt.Y)
		return true
	})

	meanX, err := safeMath(stats.Mean(X))
	if err != nil {
		return x, y, errors.Wrap(err, "unable to calculate mean of X values on given point cloud")
	}
	x.mean = meanX

	stdevX, err := safeMath(stats.StandardDeviation(X))
	if err != nil {
		return x, y, errors.Wrap(err, "unable to calculate stdev of Y values on given point cloud")
	}
	x.stdev = stdevX

	meanY, err := safeMath(stats.Mean(Y))
	if err != nil {
		return x, y, errors.Wrap(err, "unable to calculate mean of Y values on given point cloud")
	}
	y.mean = meanY

	stdevY, err := safeMath(stats.StandardDeviation(Y))
	if err != nil {
		return x, y, errors.Wrap(err, "unable to calculate stdev of Y values on given point cloud")
	}
	y.stdev = stdevY

	return x, y, nil
}

// Calculates the scaling factor needed to fit the projected pointcloud to the desired image size, cropping it
// if needed based on the mean and standard deviation of the X and Y coordinates.
func calculateScaleFactor(xRange, yRange float64) float64 {
	var scaleFactor float64
	if xRange != 0 || yRange != 0 {
		widthScaleFactor := float64(imageWidth-1) / xRange
		heightScaleFactor := float64(imageHeight-1) / yRange
		scaleFactor = math.Min(widthScaleFactor, heightScaleFactor)
	}
	return scaleFactor
}

// Errors out if overflow has occurred in the given variable or if it is NaN.
func safeMath(v float64, err error) (float64, error) {
	if err != nil {
		return 0, err
	}
	switch {
	case math.IsInf(v, 0):
		return 0, errors.New("overflow detected")
	case math.IsNaN(v):
		return 0, errors.New("NaN detected")
	}
	return v, nil
}

func flipY(y int) int {
	return imageHeight - y
}

// this color map is greyscale. The color map is being used map probability values of a PCD
// into different color buckets provided by the color map.
// generated with: https://grayscale.design/app
// Intended to match the remote-control frontend's slam 2d renderer
// component's color scheme.
var colorMap = []rimage.Color{
	rimage.NewColor(240, 240, 240),
	rimage.NewColor(220, 220, 220),
	rimage.NewColor(200, 200, 200),
	rimage.NewColor(190, 190, 190),
	rimage.NewColor(170, 170, 170),
	rimage.NewColor(150, 150, 150),
	rimage.NewColor(40, 40, 40),
	rimage.NewColor(20, 20, 20),
	rimage.NewColor(10, 10, 10),
	rimage.Black,
}

// Map the color of a pixel to a color bucket value.
func probToColorMapBucket(probability uint8, numBuckets int) int {
	prob := math.Max(math.Min(100, float64(probability)), 0)
	return int(math.Floor(float64(numBuckets-1) * prob / 100))
}

// Find the desired color bucket for a given probability. This assumes the probability will be a value from 0 to 100.
func colorBucket(probability uint8) rimage.Color {
	bucket := probToColorMapBucket(probability, len(colorMap))
	return colorMap[bucket]
}
