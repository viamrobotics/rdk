package transform

import (
	"fmt"
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

// ParallelProjectionOntoXZWithRobotMarker allows the projection of a marker onto the 2D pointcloud
// projection. Currently used by slam to display a map and robot
type ParallelProjectionOntoXZWithRobotMarker struct {
	robotPose *spatialmath.Pose
}

var (
	sigmaLevel    = 7
	imageHeight   = 300
	imageWidth    = 480
	missThreshold = 0.44
	hitThreshold  = 0.53
	voxelSize     = 2
)

// PointCloudToRGBD assumes the x,y coordinates are the same as the x,y pixels.
func (ppRM *ParallelProjectionOntoXZWithRobotMarker) PointCloudToRGBD(cloud pointcloud.PointCloud,
) (*rimage.Image, *rimage.DepthMap, error) {

	meta := cloud.MetaData()
	fmt.Printf("SIZE OF POINTCLOUD: %v", cloud.Size())

	// Calculate max and min range to be represented by the produced image using the mean and standard
	// deviation of the X and Z coordinates
	var X, Z []float64
	cloud.Iterate(0, 0, func(pt r3.Vector, data pointcloud.Data) bool {
		X = append(X, pt.X)
		Z = append(Z, pt.Z)
		return true
	})

	meanX, stdevX, err := calculateMeanAndStandardDeviation(X)
	if err != nil {
		return nil, nil, err
	}
	meanZ, stdevZ, err := calculateMeanAndStandardDeviation(Z)
	if err != nil {
		return nil, nil, err
	}

	// TODO: add overflow check
	maxX := math.Min(meanX+float64(sigmaLevel)*stdevX, meta.MaxX)
	minX := math.Max(meanX-float64(sigmaLevel)*stdevX, meta.MinX)
	maxZ := math.Min(meanZ+float64(sigmaLevel)*stdevZ, meta.MaxZ)
	minZ := math.Max(meanZ+float64(sigmaLevel)*stdevZ, meta.MinZ)

	// Change max and min values to ensure the robot marker is in the image
	var robotMarker spatialmath.Pose
	if ppRM.robotPose != nil {
		robotMarker = *ppRM.robotPose
		maxX = math.Max(maxX, robotMarker.Point().X)
		minX = math.Min(minX, robotMarker.Point().X)
		maxZ = math.Max(maxZ, robotMarker.Point().Z)
		minZ = math.Min(minZ, robotMarker.Point().Z)
	}

	// Calculate scale factor
	widthScaleFactor := float64(imageWidth) / (maxX - minX)
	heightScaleFactor := float64(imageHeight) / (maxZ - minZ)

	color := rimage.NewImage(imageWidth, imageHeight)

	cloud.Iterate(0, 0, func(pt r3.Vector, data pointcloud.Data) bool {
		j := (pt.X - meta.MinX) * widthScaleFactor
		i := (pt.Z - meta.MinZ) * heightScaleFactor
		x, y := int(math.Round(j)), int(math.Round(i))

		// if point has a value and is inside the RGB image bounds, add it to the images
		if x >= 0 && x < imageWidth && y >= 0 && y < imageHeight && data != nil {
			if data.HasValue() {
				r, g, b := getProbabilityColorFromValue(data.Value())
				addVoxelToImage(color, image.Point{x, y}, rimage.NewColor(r, g, b), voxelSize)
			} else {
				addVoxelToImage(color, image.Point{x, y}, rimage.NewColor(255, 255, 255), voxelSize)
			}
		}
		return true
	})

	// Add robot marker to image
	robotMarkerPoint := image.Point{
		X: int(math.Round((robotMarker.Point().X - meta.MinX) * widthScaleFactor)),
		Y: int(math.Round((robotMarker.Point().Z - meta.MinZ) * heightScaleFactor)),
	}
	robotMarkerColor := rimage.NewColor(255, 0, 0)

	addVoxelToImage(color, robotMarkerPoint, robotMarkerColor, voxelSize)

	return color, nil, nil
}

// RGBDToPointCloud calls its equivalent in ParallelProjection
func (ppRM *ParallelProjectionOntoXZWithRobotMarker) RGBDToPointCloud(
	img *rimage.Image,
	dm *rimage.DepthMap,
	crop ...image.Rectangle,
) (pointcloud.PointCloud, error) {
	pp := ParallelProjection{}
	return pp.RGBDToPointCloud(img, dm, crop...)
}

// ImagePointTo3DPoint calls its equivalent in ParallelProjection
func (ppRM *ParallelProjectionOntoXZWithRobotMarker) ImagePointTo3DPoint(pt image.Point, d rimage.Depth) (r3.Vector, error) {
	pp := ParallelProjection{}
	return pp.ImagePointTo3DPoint(pt, d)
}

// getProbabilityColorFromValue returns the r, g, b color values based on the the probability value and defined thresholds
func getProbabilityColorFromValue(v int) (uint8, uint8, uint8) {
	var r, g, b uint8

	prob := float64(v) / 100.

	if prob < missThreshold {
		b = uint8(255 * (prob / missThreshold))
	}
	if prob < missThreshold {
		b = uint8(255 * ((prob - hitThreshold) / (1 - hitThreshold)))
	}
	return r, g, b
}

func calculateMeanAndStandardDeviation(data []float64) (float64, float64, error) {
	mean, err := stats.Mean(data)
	if err != nil {
		return 0, 0, err
	}
	stdev, err := stats.StandardDeviation(data)
	if err != nil {
		return 0, 0, err
	}
	return mean, stdev, nil
}

func addVoxelToImage(im *rimage.Image, p image.Point, color rimage.Color, vSize int) {
	for i := int(-vSize / 2); i <= int(vSize/2); i++ {
		for j := int(-vSize / 2); j <= int(vSize/2); j++ {
			if p.X+i >= 0 && p.X+i < imageWidth && p.Y+j >= 0 && p.Y+j < imageHeight {
				im.Set(image.Point{p.X + i, p.Y + j}, color)
			}
		}
	}
}

func NewParallelProjectionOntoXZWithRobotMarker(rp *spatialmath.Pose) ParallelProjectionOntoXZWithRobotMarker {
	return ParallelProjectionOntoXZWithRobotMarker{robotPose: rp}
}
