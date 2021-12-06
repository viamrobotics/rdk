package transform

import (
	"image"

	"go.viam.com/core/pointcloud"
	"go.viam.com/core/rimage"

	"github.com/go-errors/errors"
	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
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

// NewPinholeCameraHomography takes in a struct that stores raw data from JSON and converts it into a PinholeCameraHomography struct
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

// AlignImageWithDepth will take the depth and the color image and overlay the two properly.
func (dch *PinholeCameraHomography) AlignImageWithDepth(ii *rimage.ImageWithDepth) (*rimage.ImageWithDepth, error) {
	if ii.IsAligned() {
		return rimage.MakeImageWithDepth(ii.Color, ii.Depth, true, dch), nil
	}
	if ii.Color == nil {
		return nil, errors.New("no color image present to align")
	}
	if ii.Depth == nil {
		return nil, errors.New("no depth image present to align")
	}
	// rotate depth image if necessary
	if dch.RotateDepth != 0. {
		ii.Depth = ii.Depth.Rotate(dch.RotateDepth)
	}
	// make a new depth map that is as big as the color image
	width, height := ii.Color.Width(), ii.Color.Height()
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
			depthVal := rimage.NearestNeighborDepth(depthPt, ii.Depth)
			if depthVal != nil {
				newDepth.Set(x, y, *depthVal)
			}
		}
	}
	return rimage.MakeImageWithDepth(ii.Color, newDepth, true, dch), nil
}

// ImageWithDepthToPointCloud takes an ImageWithDepth and uses the camera parameters to project it to a pointcloud.
func (dch *PinholeCameraHomography) ImageWithDepthToPointCloud(ii *rimage.ImageWithDepth) (pointcloud.PointCloud, error) {
	return colorIntrinsics2DTo3D(ii, dch.ColorCamera)
}

// PointCloudToImageWithDepth takes a PointCloud with color info and returns an ImageWithDepth from the perspective of the color camera frame.
func (dch *PinholeCameraHomography) PointCloudToImageWithDepth(cloud pointcloud.PointCloud) (*rimage.ImageWithDepth, error) {
	iwd, err := colorIntrinsics3DTo2D(cloud, dch.ColorCamera)
	if err != nil {
		return nil, err
	}
	iwd.SetCameraSystem(dch)
	return iwd, nil
}

// ImagePointTo3DPoint takes in a image coordinate and returns the 3D point from the perspective of the color camera.
func (dch *PinholeCameraHomography) ImagePointTo3DPoint(pt image.Point, iwd *rimage.ImageWithDepth) (r3.Vector, error) {
	return colorIntrinsics2DPtTo3DPt(pt, iwd, dch.ColorCamera)
}
