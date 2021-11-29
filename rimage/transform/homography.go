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
	width, height := ii.Color.Width(), ii.Color.Height()
	newDepth := rimage.NewEmptyDepthMap(width, height)
	colorToDepth := dch.DepthToColor.Inverse()
	// iterate through color pixels - use reverse homography to see where they land in the depth map.
	// use interpolation to get the depth value at that point
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			depthPt := colorToDepth.Apply(r2.Point{float64(x), float64(y)})
			depthVal := BilinearInterpolationDepth(depthPt, ii.Depth)
			if depthVal != nil {
				newDepth.Set(x, y, *depthVal)
			}
		}
	}
	return rimage.MakeImageWithDepth(ii.Color, newDepth, true, dch), nil
}
