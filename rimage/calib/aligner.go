package calib

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"go.viam.com/robotcore/pointcloud"
	"go.viam.com/robotcore/rimage"
)

type DepthColorAligner interface {
	AlignImageWithDepth(*rimage.ImageWithDepth) (*rimage.ImageWithDepth, error)
	ImageWithDepthToPointCloud(*rimage.ImageWithDepth) (*pointcloud.PointCloud, error)
	PointCloudToImageWithDepth(*pointcloud.PointCloud) (*rimage.ImageWithDepth, error)
}

// Placeholder aligner that doesn't actually do any alignment.
// Useful if you haven't developed an aligner yet, but the function parameters call for one
// WARNING  IF YOU USE THIS FOR ANYTHING REAL ONLY SUFFERING WILL FOLLOW
type DummyAligner struct{}

func NewDummyAligner() (*DummyAligner, error) {
	return &DummyAligner{}, nil
}

// Doesn't do anything, just returns a copy of the same ImageWithDepth but calls it aligned
func (da *DummyAligner) AlignImageWithDepth(iwd *rimage.ImageWithDepth) (*rimage.ImageWithDepth, error) {
	if iwd.Color == nil {
		return nil, fmt.Errorf("no color image present to align")
	}
	if iwd.Depth == nil {
		return nil, fmt.Errorf("no depth image present to align")
	}
	return rimage.MakeImageWithDepth(iwd.Color, iwd.Depth, true), nil
}

// Turns the image into a pointcloud with no regard to alignment of the images.
func (da *DummyAligner) ImageWithDepthToPointCloud(iwd *rimage.ImageWithDepth) (*pointcloud.PointCloud, error) {
	pc := pointcloud.New()

	height := iwd.Height()
	width := iwd.Width()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			z := iwd.Depth.GetDepth(x, y)
			if z == 0 {
				continue
			}
			c := iwd.Color.GetXY(x, y)
			r, g, b := c.RGB255()
			err := pc.Set(pointcloud.NewColoredPoint(float64(x), float64(y), float64(z), color.NRGBA{r, g, b, 255}))
			if err != nil {
				return nil, err
			}
		}
	}
	return pc, nil
}

// Functions flattens the pointcloud down to an image with no regard to alignment
func (da *DummyAligner) PointCloudToImageWithDepth(cloud *pointcloud.PointCloud) (*rimage.ImageWithDepth, error) {
	width := int(math.Round(cloud.MaxX()-cloud.MinX())) + 1
	height := int(math.Round(cloud.MaxY()-cloud.MinY())) + 1
	color := rimage.NewImage(width, height)
	depth := rimage.NewEmptyDepthMap(width, height)
	var err error
	cloud.Iterate(func(pt pointcloud.Point) bool {
		j := pt.Position().X - cloud.MinX()
		i := pt.Position().Y - cloud.MinY()
		x, y := int(math.Round(j)), int(math.Round(i))
		z := int(math.Round(pt.Position().Z))
		// set depth and color
		var r, g, b uint8
		if pt.HasColor() {
			r, g, b = pt.RGB255()
		} else {
			r, g, b = 0, 0, 0
		}
		if x < 0 && x >= width && y < 0 && y >= height {
			err = fmt.Errorf("cannot set point (%d,%d) out of bounds of image with dim (%d,%d)", x, y, width, height)
			return false
		}
		//fmt.Printf("Point (%d,%d)\n", x, y)
		color.Set(image.Point{x, y}, rimage.NewColor(r, g, b))
		depth.Set(x, y, rimage.Depth(z))
		return true
	})
	if err != nil {
		return nil, err
	}
	return rimage.MakeImageWithDepth(color, &depth, true), nil

}
