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
	sigmaLevel       = 7
	imageHeight      = 1080
	imageWidth       = 1080
	missThreshold    = 0.44
	hitThreshold     = 0.53
	voxelRadius      = 1
	robotMarkerRatio = 5
)

// PointCloudToRGBD creates an image of a pointcloud in the XZ plane, scaling the points to a standard image
// size. It will also add a red marker to the map to represent the location of the robot. The returned depthMap
// is unused and so will always be nil.
func (ppRM *ParallelProjectionOntoXZWithRobotMarker) PointCloudToRGBD(cloud pointcloud.PointCloud,
) (*rimage.Image, *rimage.DepthMap, error) {
	meta := cloud.MetaData()

	var X, Z []float64
	cloud.Iterate(0, 0, func(pt r3.Vector, data pointcloud.Data) bool {
		X = append(X, pt.X)
		Z = append(Z, pt.Z)
		return true
	})

	// Calculate max and min range to be represented in the output image and, if needed, cropping it based on
	// the mean and standard deviation of the X and Z coordinates
	meanX, err := stats.Mean(X)
	if err != nil {
		return nil, nil, errors.Wrap(err, "calculation of the X-coord's mean during pcd projection failed")
	}
	stdevX, err := stats.StandardDeviation(X)
	if err != nil {
		return nil, nil, errors.Wrap(err, "calculation of the X-coord's stdev during pcd projection failed")
	}

	meanZ, err := stats.Mean(Z)
	if err != nil {
		return nil, nil, errors.Wrap(err, "calculation of the Z-coord's mean during pcd projection failed")
	}
	stdevZ, err := stats.StandardDeviation(Z)
	if err != nil {
		return nil, nil, errors.Wrap(err, "calculation of the Z-coord's stdev during pcd projection failed")
	}

	maxX := math.Min(meanX+float64(sigmaLevel)*stdevX, meta.MaxX)
	minX := math.Max(meanX-float64(sigmaLevel)*stdevX, meta.MinX)
	maxZ := math.Min(meanZ+float64(sigmaLevel)*stdevZ, meta.MaxZ)
	minZ := math.Max(meanZ-float64(sigmaLevel)*stdevZ, meta.MinZ)

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
	widthScaleFactor := float64(imageWidth-1) / (maxX - minX)
	heightScaleFactor := float64(imageHeight-1) / (maxZ - minZ)

	// Add points in the pointcloud to a new image
	im := rimage.NewImage(imageWidth, imageHeight)
	cloud.Iterate(0, 0, func(pt r3.Vector, data pointcloud.Data) bool {
		j := (pt.X - minX) * widthScaleFactor
		i := (pt.Z - minZ) * heightScaleFactor
		x, y := int(math.Round(j)), int(math.Round(i))

		// Adds a point to an image using the value to define the color. If no value is available,
		// the default color of white is used.
		if x >= 0 && x < imageWidth && y >= 0 && y < imageHeight && data != nil {
			var c rimage.Color
			if data.HasValue() {
				c = getProbabilityColorFromValue(data.Value())
			} else {
				c = rimage.NewColor(255, 255, 255)
			}
			im.Circle(image.Point{x, y}, voxelRadius, c)
		}
		return true
	})

	// Add a red robot marker to the image
	if ppRM.robotPose != nil {
		robotMarkerPoint := image.Point{
			X: int(math.Round((robotMarker.Point().X - minX) * widthScaleFactor)),
			Y: int(math.Round((robotMarker.Point().Z - minZ) * heightScaleFactor)),
		}
		robotMarkerColor := rimage.NewColor(255, 0, 0)
		im.Circle(robotMarkerPoint, voxelRadius, robotMarkerColor)
	}
	return im, nil, nil
}

// RGBDToPointCloud calls its equivalent in ParallelProjection.
func (ppRM *ParallelProjectionOntoXZWithRobotMarker) RGBDToPointCloud(
	img *rimage.Image,
	dm *rimage.DepthMap,
	crop ...image.Rectangle,
) (pointcloud.PointCloud, error) {
	return nil, errors.New("converting a RGB image to Pointcloud is current unimplemented for this projection")
}

// ImagePointTo3DPoint calls its equivalent in ParallelProjection.
func (ppRM *ParallelProjectionOntoXZWithRobotMarker) ImagePointTo3DPoint(pt image.Point, d rimage.Depth) (r3.Vector, error) {
	return r3.Vector{}, errors.New("converting an image point to a 3D point is current unimplemented for this projection")
}

// getProbabilityColorFromValue returns an RGB color value based on the the probability value and defined hit and miss
// thresholds
// TODO (RSDK-1705): Once probability values are available this function should be changed to produced desired images.
func getProbabilityColorFromValue(v int) rimage.Color {
	var r, g, b uint8

	prob := float64(v) / 100.

	switch {
	case prob < missThreshold && prob >= 0:
		b = uint8(255 * (prob / missThreshold))
	case prob > hitThreshold && prob <= 1:
		g = uint8(255 * ((prob - hitThreshold) / (1 - hitThreshold)))
	default:
		r = 255
		b = 255
		g = 255
	}

	return rimage.NewColor(r, g, b)
}

// NewParallelProjectionOntoXZWithRobotMarker creates a new ParallelProjectionOntoXZWithRobotMarker with the given
// robot pose, if.
func NewParallelProjectionOntoXZWithRobotMarker(rp *spatialmath.Pose) ParallelProjectionOntoXZWithRobotMarker {
	return ParallelProjectionOntoXZWithRobotMarker{robotPose: rp}
}
