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

// ParallelProjectionOntoXZWithRobotMarker allows the creation of a 2D projection of a pointcloud and robot
// position onto the XZ plane.
type ParallelProjectionOntoXZWithRobotMarker struct {
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

// PointCloudToRGBD creates an image of a pointcloud in the XZ plane, scaling the points to a standard image
// size. It will also add a red marker to the map to represent the location of the robot. The returned depthMap
// is unused and so will always be nil.
func (ppRM *ParallelProjectionOntoXZWithRobotMarker) PointCloudToRGBD(cloud pointcloud.PointCloud,
) (*rimage.Image, *rimage.DepthMap, error) {
	meta := cloud.MetaData()

	if cloud.Size() == 0 {
		return nil, nil, errors.New("projection point cloud is empty")
	}

	meanStdevX, meanStdevZ, err := calculatePointCloudMeanAndStdevXZ(cloud)
	if err != nil {
		return nil, nil, err
	}

	maxX := math.Min(meanStdevX.mean+float64(sigmaLevel)*meanStdevZ.stdev, meta.MaxX)
	minX := math.Max(meanStdevX.mean-float64(sigmaLevel)*meanStdevZ.stdev, meta.MinX)
	maxZ := math.Min(meanStdevZ.mean+float64(sigmaLevel)*meanStdevZ.stdev, meta.MaxZ)
	minZ := math.Max(meanStdevZ.mean-float64(sigmaLevel)*meanStdevZ.stdev, meta.MinZ)

	// Change the max and min values to ensure the robot marker can be represented in the output image
	var robotMarker spatialmath.Pose
	if ppRM.robotPose != nil {
		robotMarker = *ppRM.robotPose
		maxX = math.Max(maxX, robotMarker.Point().X)
		minX = math.Min(minX, robotMarker.Point().X)
		maxZ = math.Max(maxZ, robotMarker.Point().Z)
		minZ = math.Min(minZ, robotMarker.Point().Z)
	}

	// Calculate the scale factors
	scaleFactor := calculateScaleFactor(maxX-minX, maxZ-minZ)

	// Add points in the pointcloud to a new image
	var pointColor rimage.Color
	im := rimage.NewImage(imageWidth, imageHeight)
	cloud.Iterate(0, 0, func(pt r3.Vector, data pointcloud.Data) bool {
		x := int(math.Round((pt.X - minX) * scaleFactor))
		y := int(math.Round((pt.Z - minZ) * scaleFactor))

		// Adds a point to an image using the value to define the color. If no value is available,
		// the default color of white is used.
		if x >= 0 && x < imageWidth && y >= 0 && y < imageHeight {
			pointColor, err = getColorFromProbabilityValue(data)
			if err != nil {
				return false
			}
			im.Circle(image.Point{X: x, Y: y}, pointRadius, pointColor)
		}
		return true
	})

	if err != nil {
		return nil, nil, err
	}

	// Add a red robot marker to the image
	if ppRM.robotPose != nil {
		x := int(math.Round((robotMarker.Point().X - minX) * scaleFactor))
		y := int(math.Round((robotMarker.Point().Z - minZ) * scaleFactor))
		robotMarkerColor := rimage.NewColor(255, 0, 0)
		im.Circle(image.Point{X: x, Y: y}, robotMarkerRadius, robotMarkerColor)
	}
	return im, nil, nil
}

// RGBDToPointCloud is unimplemented and will produce an error.
func (ppRM *ParallelProjectionOntoXZWithRobotMarker) RGBDToPointCloud(
	img *rimage.Image,
	dm *rimage.DepthMap,
	crop ...image.Rectangle,
) (pointcloud.PointCloud, error) {
	return nil, errors.New("converting an RGB image to Pointcloud is currently unimplemented for this projection")
}

// ImagePointTo3DPoint is unimplemented and will produce an error.
func (ppRM *ParallelProjectionOntoXZWithRobotMarker) ImagePointTo3DPoint(pt image.Point, d rimage.Depth) (r3.Vector, error) {
	return r3.Vector{}, errors.New("converting an image point to a 3D point is currently unimplemented for this projection")
}

// getColorFromProbabilityValue returns an RGB color value based on the probability value and defined hit and miss
// thresholds
// TODO (RSDK-1705): Once probability values are available, a temporary algorithm is being used based on Cartographer's method
// of painting images. Currently this function will return a shade of green if the probability is above the hit threshold and
// a shade of blue if it is below the miss threshold. These shades will be more distinct the further from the threshold they are.
func getColorFromProbabilityValue(d pointcloud.Data) (rimage.Color, error) {
	var r, g, b uint8

	if d == nil {
		return rimage.NewColor(0, 0, 0), errors.New("data received was null")
	}

	if !d.HasValue() {
		return rimage.NewColor(255, 255, 255), nil
	}

	if d.Value() > 100 || d.Value() < 0 {
		return rimage.NewColor(0, 0, 0),
			errors.Errorf("received a value of %v which is outside the range (0 - 100) representing probabilities", d.Value())
	}

	prob := float64(d.Value()) / 100.

	switch {
	case prob < missThreshold:
		b = uint8(255 * ((missThreshold - prob) / (hitThreshold - 0)))
	case prob > hitThreshold:
		g = uint8(255 * ((prob - hitThreshold) / (1 - missThreshold)))
	default:
		b = uint8(255 * ((missThreshold - prob) / (hitThreshold - 0)))
		g = uint8(255 * ((prob - hitThreshold) / (1 - missThreshold)))
	}

	return rimage.NewColor(r, g, b), nil
}

// NewParallelProjectionOntoXZWithRobotMarker creates a new ParallelProjectionOntoXZWithRobotMarker with the given
// robot pose.
func NewParallelProjectionOntoXZWithRobotMarker(rp *spatialmath.Pose) ParallelProjectionOntoXZWithRobotMarker {
	return ParallelProjectionOntoXZWithRobotMarker{robotPose: rp}
}

// Struct containing the mean and stdev.
type meanStdev struct {
	mean  float64
	stdev float64
}

// Calculates the mean and standard deviation of the X and Z coordinates stored in the point cloud.
func calculatePointCloudMeanAndStdevXZ(cloud pointcloud.PointCloud) (meanStdev, meanStdev, error) {
	var X, Z []float64
	var x, z meanStdev

	cloud.Iterate(0, 0, func(pt r3.Vector, data pointcloud.Data) bool {
		X = append(X, pt.X)
		Z = append(Z, pt.Z)
		return true
	})

	meanX, err := safeMath(stats.Mean(X))
	if err != nil {
		return x, z, errors.Wrap(err, "unable to calculate mean of X values on given point cloud")
	}
	x.mean = meanX

	stdevX, err := safeMath(stats.StandardDeviation(X))
	if err != nil {
		return x, z, errors.Wrap(err, "unable to calculate stdev of Z values on given point cloud")
	}
	x.stdev = stdevX

	meanZ, err := safeMath(stats.Mean(Z))
	if err != nil {
		return x, z, errors.Wrap(err, "unable to calculate mean of Z values on given point cloud")
	}
	z.mean = meanZ

	stdevZ, err := safeMath(stats.StandardDeviation(Z))
	if err != nil {
		return x, z, errors.Wrap(err, "unable to calculate stdev of Z values on given point cloud")
	}
	z.stdev = stdevZ

	return x, z, nil
}

// Calculates the scaling factor needed to fit the projected pointcloud to the desired image size, cropping it
// if needed based on the mean and standard deviation of the X and Z coordinates.
func calculateScaleFactor(xRange, zRange float64) float64 {
	var scaleFactor float64
	if xRange != 0 || zRange != 0 {
		widthScaleFactor := float64(imageWidth-1) / xRange
		heightScaleFactor := float64(imageHeight-1) / zRange
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
