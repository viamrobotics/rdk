package transform

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"

	"go.viam.com/core/pointcloud"
	"go.viam.com/core/rimage"
)

// Function to take an unaligned ImageWithDepth and align it, returning a new ImageWithDepth.
func (dcie *DepthColorIntrinsicsExtrinsics) AlignImageWithDepth(ii *rimage.ImageWithDepth) (*rimage.ImageWithDepth, error) {
	if ii.IsAligned() {
		return rimage.MakeImageWithDepth(ii.Color, ii.Depth, true, dcie), nil
	}
	if ii.Color == nil {
		return nil, errors.New("no color image present to align")
	}
	if ii.Depth == nil {
		return nil, errors.New("no depth image present to align")
	}
	newImgWithDepth, err := dcie.TransformDepthCoordToColorCoord(ii)
	if err != nil {
		return nil, err
	}
	return newImgWithDepth, nil
}

// Function that changes the coordinate system of the depth map to be in same coordinate system
// as the color image
func (dcie *DepthColorIntrinsicsExtrinsics) TransformDepthCoordToColorCoord(img *rimage.ImageWithDepth) (*rimage.ImageWithDepth, error) {
	if img.Color.Height() != dcie.ColorCamera.Height || img.Color.Width() != dcie.ColorCamera.Width {
		return nil, fmt.Errorf("camera matrices expected color image of (%#v,%#v), got (%#v, %#v)", dcie.ColorCamera.Width, dcie.ColorCamera.Height, img.Color.Width(), img.Color.Height())
	}
	if img.Depth.Height() != dcie.DepthCamera.Height || img.Depth.Width() != dcie.DepthCamera.Width {
		return nil, fmt.Errorf("camera matrices expected depth image of (%#v,%#v), got (%#v, %#v)", dcie.DepthCamera.Width, dcie.DepthCamera.Height, img.Depth.Width(), img.Depth.Height())
	}
	inmap := img.Depth
	outmap := rimage.NewEmptyDepthMap(dcie.ColorCamera.Width, dcie.ColorCamera.Height)
	for dy := 0; dy < dcie.DepthCamera.Height; dy++ {
		for dx := 0; dx < dcie.DepthCamera.Width; dx++ {
			dz := inmap.GetDepth(dx, dy)
			if dz == 0 {
				continue
			}
			// if depth pixels are bigger than color pixel, will cause a grid effect. Take into account size of pixel
			// get top-left corner of depth pixel
			cx, cy, cz0 := dcie.DepthPixelToColorPixel(float64(dx)-0.5, float64(dy)-0.5, float64(dz))
			cx0, cy0 := int(cx+0.5), int(cy+0.5)
			// get bottom-right corner of depth pixel
			cx, cy, cz1 := dcie.DepthPixelToColorPixel(float64(dx)+0.5, float64(dy)+0.5, float64(dz))
			cx1, cy1 := int(cx+0.5), int(cy+0.5)
			if cx0 < 0 || cy0 < 0 || cx1 > dcie.ColorCamera.Width-1 || cy1 > dcie.ColorCamera.Height-1 {
				continue
			}
			z := rimage.Depth((cz0 + cz1) / 2.0) // average of depth within color pixel
			for y := cy0; y <= cy1; y++ {
				for x := cx0; x <= cx1; x++ {
					outmap.Set(x, y, z)
				}
			}
		}
	}
	return rimage.MakeImageWithDepth(img.Color, outmap, true, dcie), nil
}

// Function that takes an ImageWithDepth and uses the camera parameters to project it to a pointcloud.
// Aligns it if it isn't already aligned.
func (dcie *DepthColorIntrinsicsExtrinsics) ImageWithDepthToPointCloud(ii *rimage.ImageWithDepth) (pointcloud.PointCloud, error) {
	var iwd *rimage.ImageWithDepth
	var err error
	// color and depth images need to already be aligned
	if ii.IsAligned() {
		iwd = rimage.MakeImageWithDepth(ii.Color, ii.Depth, true, dcie)
	} else {
		iwd, err = dcie.AlignImageWithDepth(ii)
		if err != nil {
			return nil, err
		}
	}
	// Check dimensions, they should be aligned to the color frame
	if iwd.Depth.Width() != iwd.Color.Width() ||
		iwd.Depth.Height() != iwd.Color.Height() {
		return nil, fmt.Errorf("depth map and color dimensions don't match %d,%d -> %d,%d",
			iwd.Depth.Width(), iwd.Depth.Height(), iwd.Color.Width(), iwd.Color.Height())
	}
	pc := pointcloud.New()

	for y := 0; y < iwd.Color.Height(); y++ {
		for x := 0; x < iwd.Color.Width(); x++ {
			px, py, pz := dcie.ColorCamera.PixelToPoint(float64(x), float64(y), float64(iwd.Depth.GetDepth(x, y)))
			r, g, b := iwd.Color.GetXY(x, y).RGB255()
			err = pc.Set(pointcloud.NewColoredPoint(px, py, pz, color.NRGBA{r, g, b, 255}))
			if err != nil {
				return nil, err
			}
		}
	}
	return pc, nil

}

// Function that takes a PointCloud with color info and returns an ImageWithDepth from the perspective of the color camera frame.
func (dcie *DepthColorIntrinsicsExtrinsics) PointCloudToImageWithDepth(cloud pointcloud.PointCloud) (*rimage.ImageWithDepth, error) {
	// Needs to be a pointcloud with color
	if !cloud.HasColor() {
		return nil, errors.New("pointcloud has no color information, cannot create an image with depth")
	}
	// ImageWithDepth will be in the camera frame of the RGB camera.
	// Points outside of the frame will be discarded.
	// Assumption is that points in pointcloud are in mm.
	width, height := dcie.ColorCamera.Width, dcie.ColorCamera.Height
	color := rimage.NewImage(width, height)
	depth := rimage.NewEmptyDepthMap(width, height)
	cloud.Iterate(func(pt pointcloud.Point) bool {
		j, i := dcie.ColorCamera.PointToPixel(pt.Position().X, pt.Position().Y, pt.Position().Z)
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
	return rimage.MakeImageWithDepth(color, depth, true, dcie), nil

}

// Converts a Depth Map to a PointCloud using the depth camera parameters
func DepthMapToPointCloud(depthImage *rimage.DepthMap, pixel2meter float64, params *PinholeCameraIntrinsics, depthMin, depthMax rimage.Depth) (pointcloud.PointCloud, error) {
	// create new point cloud
	pcOut := pointcloud.New()
	// go through depth map pixels and get 3D Points
	for y := 0; y < depthImage.Height(); y++ {
		for x := 0; x < depthImage.Width(); x++ {
			// get depth value
			d := depthImage.Get(image.Point{x, y})
			if d >= depthMin && d < depthMax {
				// get z distance to meter for unit uniformity
				z := float64(d) * pixel2meter
				// get x and y of 3D point
				xPoint, yPoint, z := params.PixelToPoint(float64(x), float64(y), z)
				// Get point in PointCloud format
				xPoint = xPoint / pixel2meter
				yPoint = yPoint / pixel2meter
				z = z / pixel2meter
				pt := pointcloud.NewBasicPoint(xPoint, yPoint, z)
				err := pcOut.Set(pt)
				if err != nil {
					err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %w", xPoint, yPoint, z, err)
					return nil, err
				}
			}
		}
	}
	return pcOut, nil
}

// Function to project 3D point in a given camera image plane
// Return new pointclouds leaving original unchanged
func ApplyRigidBodyTransform(pts pointcloud.PointCloud, params *Extrinsics) (pointcloud.PointCloud, error) {
	transformedPoints := pointcloud.New()
	var err error
	pts.Iterate(func(pt pointcloud.Point) bool {
		x, y, z := params.TransformPointToPoint(pt.Position().X, pt.Position().Y, pt.Position().Z)
		var ptTransformed pointcloud.Point
		if pt.HasColor() {
			ptTransformed = pointcloud.NewColoredPoint(x, y, z, pt.Color().(color.NRGBA))
		} else if pt.HasValue() {
			ptTransformed = pointcloud.NewValuePoint(x, y, z, pt.Value())
		} else {
			ptTransformed = pointcloud.NewBasicPoint(x, y, z)
		}
		err = transformedPoints.Set(ptTransformed)
		if err != nil {
			err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %w", x, y, z, err)
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return transformedPoints, nil
}

// Function to project points in a pointcloud to a given camera image plane
func ProjectPointCloudToRGBPlane(pts pointcloud.PointCloud, h, w int, params PinholeCameraIntrinsics, pixel2meter float64) (pointcloud.PointCloud, error) {
	coordinates := pointcloud.New()
	var err error
	pts.Iterate(func(pt pointcloud.Point) bool {
		j, i := params.PointToPixel(pt.Position().X, pt.Position().Y, pt.Position().Z)
		j = math.Round(j)
		i = math.Round(i)
		// if point has color is inside the RGB image bounds, add it to the new pointcloud
		if j >= 0 && j < float64(w) && i >= 0 && i < float64(h) && pt.HasColor() {
			pt2d := pointcloud.NewColoredPoint(j, i, pt.Position().Z, pt.Color().(color.NRGBA))
			err = coordinates.Set(pt2d)
			if err != nil {
				err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %w", j, i, pt.Position().Z, err)
				return false
			}
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return coordinates, nil
}

//TODO(louise): Add Depth Map dilation function as in the librealsense library
