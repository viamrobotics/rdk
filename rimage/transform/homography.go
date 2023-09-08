//go:build cgo
package transform

import (
	"github.com/golang/geo/r2"
	"github.com/pkg/errors"

	"go.viam.com/rdk/rimage"
)

// RawDepthColorHomography is a structure that can be easily serialized and unserialized into JSON.
type RawDepthColorHomography struct {
	Homography   []float64 `json:"transform"`
	DepthToColor bool      `json:"depth_to_color"`
	RotateDepth  int       `json:"rotate_depth_degs"`
}

// CheckValid runs checks on the fields of the struct to see if the inputs are valid.
func (rdch *RawDepthColorHomography) CheckValid() error {
	if rdch == nil {
		return errors.New("pointer to DepthColorHomography is nil")
	}
	if rdch.Homography == nil {
		return errors.New("pointer to Homography is nil")
	}
	if len(rdch.Homography) != 9 {
		return errors.Errorf("input to NewHomography must have length of 9. Has length of %d", len(rdch.Homography))
	}
	return nil
}

// DepthColorHomography stores the color camera intrinsics and the homography that aligns a depth map
// with the color image. DepthToColor is true if the homography maps from depth pixels to color pixels, and false
// if it maps from color pixels to depth pixels.
// These parameters can take the color and depth image and create a point cloud of 3D points
// where the origin is the origin of the color camera, with units of mm.
type DepthColorHomography struct {
	Homography   *Homography `json:"transform"`
	DepthToColor bool        `json:"depth_to_color"`
	RotateDepth  int         `json:"rotate_depth_degs"`
}

// NewDepthColorHomography takes in a struct that stores raw data from JSON and converts it into a DepthColorHomography struct.
func NewDepthColorHomography(inp *RawDepthColorHomography) (*DepthColorHomography, error) {
	homography, err := NewHomography(inp.Homography)
	if err != nil {
		return nil, err
	}
	return &DepthColorHomography{
		Homography:   homography,
		DepthToColor: inp.DepthToColor,
		RotateDepth:  inp.RotateDepth,
	}, nil
}

// AlignColorAndDepthImage will take the depth and the color image and overlay the two properly by transforming
// the depth map to the color map.
func (dch *DepthColorHomography) AlignColorAndDepthImage(col *rimage.Image, dep *rimage.DepthMap,
) (*rimage.Image, *rimage.DepthMap, error) {
	if col == nil {
		return nil, nil, errors.New("no color image present to align")
	}
	if dep == nil {
		return nil, nil, errors.New("no depth image present to align")
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
			return nil, nil, err
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
	return col, newDepth, nil
}
