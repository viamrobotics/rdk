package transform

import (
	"image"
	"image/color"
	"math"

	"github.com/go-errors/errors"

	"go.viam.com/core/pointcloud"
	"go.viam.com/core/rimage"

	"github.com/edaniels/golog"
)

// DepthColorWarpTransforms TODO
type DepthColorWarpTransforms struct {
	ColorTransform, DepthTransform rimage.TransformationMatrix
	*AlignConfig                   // anonymous fields
}

// ImageWithDepthToPointCloud TODO
func (dct *DepthColorWarpTransforms) ImageWithDepthToPointCloud(ii *rimage.ImageWithDepth) (pointcloud.PointCloud, error) {
	var iwd *rimage.ImageWithDepth
	var err error
	if ii.IsAligned() {
		iwd = rimage.MakeImageWithDepth(ii.Color, ii.Depth, true, dct)
	} else {
		iwd, err = dct.AlignImageWithDepth(ii)
		if err != nil {
			return nil, err
		}
	}
	// All points now in Common frame
	pc := pointcloud.New()

	height := iwd.Height()
	width := iwd.Width()
	// TODO (bijan): this is a naive projection to 3D space, implement a better algo for warp points
	// Will need more than 2 points for warp points to create better projection
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

// AlignImageWithDepth TODO
func (dct *DepthColorWarpTransforms) AlignImageWithDepth(ii *rimage.ImageWithDepth) (*rimage.ImageWithDepth, error) {
	if ii.IsAligned() {
		return rimage.MakeImageWithDepth(ii.Color, ii.Depth, true, dct), nil
	}
	if ii.Color == nil {
		return nil, errors.New("no color image present to align")
	}
	if ii.Depth == nil {
		return nil, errors.New("no depth image present to align")
	}
	if ii.Color.Width() != dct.ColorInputSize.X ||
		ii.Color.Height() != dct.ColorInputSize.Y ||
		ii.Depth.Width() != dct.DepthInputSize.X ||
		ii.Depth.Height() != dct.DepthInputSize.Y {
		return nil, errors.Errorf("unexpected aligned dimensions c:(%d,%d) d:(%d,%d) config: %#v",
			ii.Color.Width(), ii.Color.Height(), ii.Depth.Width(), ii.Depth.Height(), dct.AlignConfig)
	}

	if dct.Smooth {
		ii.Depth.Smooth() // TODO(erh): maybe instead of this I should change warp to let the user determine how to average
	}

	c2 := rimage.WarpImage(ii, dct.ColorTransform, dct.OutputSize)
	dm2 := ii.Depth.Warp(dct.DepthTransform, dct.OutputSize)

	return rimage.MakeImageWithDepth(c2, dm2, true, dct), nil
}

// PointCloudToImageWithDepth takes a PointCloud with color info and returns an ImageWithDepth from the perspective of the color camera frame.
func (dct *DepthColorWarpTransforms) PointCloudToImageWithDepth(cloud pointcloud.PointCloud) (*rimage.ImageWithDepth, error) {
	// Needs to be a pointcloud with color
	if !cloud.HasColor() {
		return nil, errors.New("pointcloud has no color information, cannot create an image with depth")
	}
	// ImageWithDepth will be in the camera frame of the RGB camera.
	// Points outside of the frame will be discarded.
	// Assumption is that points in pointcloud are in mm.
	width, height := dct.OutputSize.X, dct.OutputSize.Y
	color := rimage.NewImage(width, height)
	depth := rimage.NewEmptyDepthMap(width, height)
	//TODO(bijan): naive implementation until we get get more points in the warp config
	cloud.Iterate(func(pt pointcloud.Point) bool {
		j := pt.Position().X - cloud.MinX()
		i := pt.Position().Y - cloud.MinY()
		x, y := int(math.Round(j)), int(math.Round(i))
		z := int(pt.Position().Z)
		// if point has color and is inside the RGB image bounds, add it to the images
		if x >= 0 && x < width && y >= 0 && y < height && pt.HasColor() {
			r, g, b := pt.RGB255()
			color.Set(image.Point{x, y}, rimage.NewColor(r, g, b))
			depth.Set(x, y, rimage.Depth(z))
		}
		return true
	})
	return rimage.MakeImageWithDepth(color, depth, true, dct), nil

}

// NewDepthColorWarpTransforms TODO
func NewDepthColorWarpTransforms(config *AlignConfig, logger golog.Logger) (*DepthColorWarpTransforms, error) {
	var err error
	dst := rimage.ArrayToPoints([]image.Point{{0, 0}, {config.OutputSize.X, config.OutputSize.Y}})

	if config.WarpFromCommon {
		config, err = config.ComputeWarpFromCommon(logger)
		if err != nil {
			return nil, err
		}
	}

	colorPoints := rimage.ArrayToPoints(config.ColorWarpPoints)
	depthPoints := rimage.ArrayToPoints(config.DepthWarpPoints)

	colorTransform := rimage.GetPerspectiveTransform(colorPoints, dst)
	depthTransform := rimage.GetPerspectiveTransform(depthPoints, dst)

	return &DepthColorWarpTransforms{colorTransform, depthTransform, config}, nil
}
