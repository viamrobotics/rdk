package rimage

import (
	"image"
	"image/color"
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/pointcloud"
)

// Aligner aligns a color and depth image together.
type Aligner interface {
	AlignColorAndDepthImage(*Image, *DepthMap) (*ImageWithDepth, error)
}

// Projector can transform a scene between a 2D ImageWithDepth and a 3D pointcloud.
type Projector interface {
	ImageWithDepthToPointCloud(*ImageWithDepth) (pointcloud.PointCloud, error)
	PointCloudToImageWithDepth(pointcloud.PointCloud) (*ImageWithDepth, error)
	ImagePointTo3DPoint(image.Point, Depth) (r3.Vector, error)
}

// A CameraSystem stores the system of camera models, the intrinsic parameters of each camera,
// and the extrinsics that relate them to each other. Used for image alignment and 2D<->3D projection.
type CameraSystem interface {
	Aligner
	Projector
}

// ParallelProjection to pointclouds are done in a naive way that don't take any camera parameters into account.
// These are not great projections, and should really only be used for testing or artistic purposes.
type ParallelProjection struct{}

// ImageWithDepthToPointCloud take a 2D image with depth and project to a 3D point cloud.
func (pp *ParallelProjection) ImageWithDepthToPointCloud(ii *ImageWithDepth) (pointcloud.PointCloud, error) {
	if !ii.IsAligned() {
		return nil, errors.New("input ImageWithDepth is not aligned")
	}
	if ii.Depth == nil {
		return nil, errors.New("input ImageWithDepth has no depth channel to project")
	}
	pc := pointcloud.New()
	height := ii.Height()
	width := ii.Width()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			z := ii.Depth.GetDepth(x, y)
			if z == 0 {
				continue
			}
			c := ii.Color.GetXY(x, y)
			r, g, b := c.RGB255()
			err := pc.Set(pointcloud.NewColoredPoint(float64(x), float64(y), float64(z), color.NRGBA{r, g, b, 255}))
			if err != nil {
				return nil, err
			}
		}
	}
	return pc, nil
}

// PointCloudToImageWithDepth assumes the x,y coordinates are the same as the x,y pixels.
func (pp *ParallelProjection) PointCloudToImageWithDepth(cloud pointcloud.PointCloud) (*ImageWithDepth, error) {
	// Needs to be a pointcloud with color
	if !cloud.HasColor() {
		return nil, errors.New("pointcloud has no color information, cannot create an image with depth")
	}
	// ImageWithDepth will be in the camera frame of the RGB camera.
	// Points outside of the frame will be discarded.
	// Assumption is that points in pointcloud are in mm.
	width := int(cloud.MaxX() - cloud.MinX())
	height := int(cloud.MaxY() - cloud.MinY())
	color := NewImage(width, height)
	depth := NewEmptyDepthMap(width, height)
	cloud.Iterate(func(pt pointcloud.Point) bool {
		j := pt.Position().X - cloud.MinX()
		i := pt.Position().Y - cloud.MinY()
		x, y := int(math.Round(j)), int(math.Round(i))
		z := int(pt.Position().Z)
		// if point has color and is inside the RGB image bounds, add it to the images
		if x >= 0 && x < width && y >= 0 && y < height && pt.HasColor() {
			r, g, b := pt.RGB255()
			color.Set(image.Point{x, y}, NewColor(r, g, b))
			depth.Set(x, y, Depth(z))
		}
		return true
	})
	return MakeImageWithDepth(color, depth, true), nil
}

// ImagePointTo3DPoint takes the 2D pixel point and assumes that it represents the X,Y coordinate in mm as well.
func (pp *ParallelProjection) ImagePointTo3DPoint(pt image.Point, d Depth) (r3.Vector, error) {
	return r3.Vector{float64(pt.X), float64(pt.Y), float64(d)}, nil
}
