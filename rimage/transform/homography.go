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
	// use homography to transform depth points to color points and store in hashmap
	// interpolate pixel values
	// save new image with depth
}
