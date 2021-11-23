package transform

import (
	"go.viam.com/core/rimage"
)

// AlignImageWithDepth will take the depth and the color image and overlay the two properly.
func (dch *DepthColorHomography) AlignImageWithDepth(ii *rimage.ImageWithDepth) (*rimage.ImageWithDepth, error) {
	if ii.IsAligned() {
		return rimage.MakeImageWithDepth(ii.Color, ii.Depth, true, dct), nil
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
	// iterate through color pixels - use reverse homography to see where they land in the depth map.
	// use bilinear interpretation in the depth map to get the value.
	// set the color-sized depth map to the interpolated depth value
	// save new image with depth
}
