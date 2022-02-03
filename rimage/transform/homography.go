package transform

import (
	"image"

	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
)

// PinholeCameraHomography stores the color camera intrinsics and the homography that aligns a depth map
// with the color image. DepthToColor is true if the homography maps from depth pixels to color pixels, and false
// if it maps from color pixels to depth pixels.
// These parameters can take the color and depth image and create a point cloud of 3D points
// where the origin is the origin of the color camera, with units of mm.
type PinholeCameraHomography struct {
	ColorCamera  PinholeCameraIntrinsics `json:"color"`
	Homography   *Homography             `json:"transform"`
	DepthToColor bool                    `json:"depth_to_color"`
	RotateDepth  int                     `json:"rotate_depth"`
}

// NewPinholeCameraHomography takes in a struct that stores raw data from JSON and converts it into a PinholeCameraHomography struct.
func NewPinholeCameraHomography(inp *RawPinholeCameraHomography) (*PinholeCameraHomography, error) {
	homography, err := NewHomography(inp.Homography)
	if err != nil {
		return nil, err
	}
	return &PinholeCameraHomography{
		ColorCamera:  inp.ColorCamera,
		Homography:   homography,
		DepthToColor: inp.DepthToColor,
		RotateDepth:  inp.RotateDepth,
	}, nil
}

// AlignColorAndDepthImage will take the depth and the color image and overlay the two properly.
func (dch *PinholeCameraHomography) AlignColorAndDepthImage(col *rimage.Image, dep *rimage.DepthMap) (*rimage.ImageWithDepth, error) {
	if col == nil {
		return nil, errors.New("no color image present to align")
	}
	if dep == nil {
		return nil, errors.New("no depth image present to align")
	}
	if col.Width() != dch.ColorCamera.Width || col.Height() != dch.ColorCamera.Height {
		return nil, errors.Errorf(
			"dimension of color image (%d, %d) does not match color camera parameters (%d, %d)",
			col.Width(),
			col.Height(),
			dch.ColorCamera.Width,
			dch.ColorCamera.Height,
		)
	}
	// rotate depth image if necessary
	if dch.RotateDepth != 0. {
		dep = dep.Rotate(dch.RotateDepth)
	}
	// make a new depth map that is as big as the color image
	width, height := col.Width(), col.Height()
	newDepth := rimage.NewEmptyDepthMap(width, height)
	// get the homography that will turn color pixels into depth pixels
	var err error
	colorToDepth := dch.Homography
	if dch.DepthToColor {
		colorToDepth, err = dch.Homography.Inverse()
		if err != nil {
			return nil, err
		}
	}
	// iterate through color pixels - use the homography to see where they land in the depth map.
	// use interpolation to get the depth value at that point
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			depthPt := colorToDepth.Apply(r2.Point{float64(x), float64(y)})
			depthVal := rimage.NearestNeighborDepth(depthPt, dep)
			if depthVal != nil {
				newDepth.Set(x, y, *depthVal)
			}
		}
	}
	return rimage.MakeImageWithDepth(col, newDepth, true), nil
}

// ImageWithDepthToPointCloud takes an ImageWithDepth and uses the camera parameters to project it to a pointcloud.
func (dch *PinholeCameraHomography) ImageWithDepthToPointCloud(ii *rimage.ImageWithDepth) (pointcloud.PointCloud, error) {
	// if not already aligned, then align the picture
	var iwd *rimage.ImageWithDepth
	var err error
	// color and depth images need to already be aligned
	if ii.IsAligned() {
		iwd = ii
	} else {
		iwd, err = dch.AlignColorAndDepthImage(ii.Color, ii.Depth)
		if err != nil {
			return nil, err
		}
	}
	return intrinsics2DTo3D(iwd, &dch.ColorCamera)
}

// PointCloudToImageWithDepth takes a PointCloud with color info and returns an ImageWithDepth
// from the perspective of the color camera referenceframe.
func (dch *PinholeCameraHomography) PointCloudToImageWithDepth(
	cloud pointcloud.PointCloud,
) (*rimage.ImageWithDepth, error) {
	return intrinsics3DTo2D(cloud, &dch.ColorCamera)
}

// ImagePointTo3DPoint takes in a image coordinate and returns the 3D point from the perspective of the color camera.
func (dch *PinholeCameraHomography) ImagePointTo3DPoint(pt image.Point, d rimage.Depth) (r3.Vector, error) {
	return intrinsics2DPtTo3DPt(pt, d, &dch.ColorCamera)
}
