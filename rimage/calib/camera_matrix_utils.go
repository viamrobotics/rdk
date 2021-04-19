package calib

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"go.viam.com/robotcore/pointcloud"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/utils"
)

// change coordinate system of depth map to be in same coordinate system as color image
// then crop both images
func (dcie *DepthColorIntrinsicsExtrinsics) TransformDepthCoordToColorCoord(img *rimage.ImageWithDepth) (*rimage.ImageWithDepth, error) {
	if img.Color.Height() != dcie.ColorCamera.Height || img.Color.Width() != dcie.ColorCamera.Width {
		return nil, fmt.Errorf("camera matrices expected color image of (%#v,%#v), got (%#v, %#v)", dcie.ColorCamera.Width, dcie.ColorCamera.Height, img.Color.Width(), img.Color.Height())
	}
	if img.Depth.Height() != dcie.DepthCamera.Height || img.Depth.Width() != dcie.DepthCamera.Width {
		return nil, fmt.Errorf("camera matrices expected depth image of (%#v,%#v), got (%#v, %#v)", dcie.DepthCamera.Width, dcie.DepthCamera.Height, img.Depth.Width(), img.Depth.Height())
	}
	inmap := img.Depth
	// keep track of the bounds of the new depth image, then use these to crop the image
	xMin, xMax, yMin, yMax := dcie.ColorCamera.Width, 0, dcie.ColorCamera.Height, 0
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
			xMin, yMin = utils.MinInt(xMin, cx0), utils.MinInt(yMin, cy0)
			xMax, yMax = utils.MaxInt(xMax, cx1), utils.MaxInt(yMax, cy1)
			z := rimage.Depth((cz0 + cz1) / 2.0) // average of depth within color pixel
			for y := cy0; y <= cy1; y++ {
				for x := cx0; x <= cx1; x++ {
					outmap.Set(x, y, z)
				}
			}
		}
	}
	crop := image.Rect(xMin, yMin, xMax, yMax)
	outmap = outmap.SubImage(crop)
	outcol := img.Color.SubImage(crop)
	return rimage.MakeImageWithDepth(&outcol, &outmap, true), nil
}

func (dcie *DepthColorIntrinsicsExtrinsics) AlignedImageToPointCloud(iwd *rimage.ImageWithDepth) (*pointcloud.PointCloud, error) {
	// color and depth images need to already be aligned, check dimensions
	// They are also aligned to the color frame
	if iwd.Depth.Width() != iwd.Color.Width() ||
		iwd.Depth.Height() != iwd.Color.Height() {
		return nil, fmt.Errorf("depth map and color dimensions don't match %d,%d -> %d,%d",
			iwd.Depth.Width(), iwd.Depth.Height(), iwd.Color.Width(), iwd.Color.Height())
	}
	pc := pointcloud.New()

	for y := 0; y < iwd.Color.Height(); y++ {
		for x := 0; x < iwd.Color.Width(); x++ {
			r, g, b := iwd.Color.GetXY(x, y).RGB255()
			px, py, pz := dcie.ColorCamera.PixelToPoint(float64(x), float64(y), float64(iwd.Depth.GetDepth(x, y)))
			err := pc.Set(pointcloud.NewColoredPoint(px, py, pz, color.NRGBA{r, g, b, 255}))
			if err != nil {
				return nil, err
			}
		}
	}
	return pc, nil

}

// Convert Depth map to point cloud (units in mm to get int coordinates) as defined in pointcloud/pointcloud.go
func DepthMapToPointCloud(depthImage *rimage.DepthMap, pixel2meter float64, params PinholeCameraIntrinsics) (*pointcloud.PointCloud, error) {
	// create new point cloud
	pcOut := pointcloud.New()
	// go through depth map pixels and get 3D Points
	for y := 0; y < depthImage.Height(); y++ {
		for x := 0; x < depthImage.Width(); x++ {
			// get depth value
			d := depthImage.Get(image.Point{x, y})
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
				err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %s", xPoint, yPoint, z, err)
				return nil, err
			}
		}
	}
	return pcOut, nil
}

// Function to project 3D point in a given camera image plane
// Return new pointclouds leaving original unchanged
func ApplyRigidBodyTransform(pts *pointcloud.PointCloud, params *Extrinsics) (*pointcloud.PointCloud, error) {
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
			err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %s", x, y, z, err)
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return transformedPoints, nil
}

// Convert point from meters (float64) to mm (int)
func MeterToDepthUnit(x, y, z float64, pixel2Meter float64) (float64, float64, float64) {
	if pixel2Meter < 0.0000001 {
		panic("pixel2Meter is too close to zero to make the conversion from meters to millimeters.")
	}
	xMm := x / pixel2Meter
	yMm := y / pixel2Meter
	zMm := z / pixel2Meter
	return xMm, yMm, zMm
}

// Function to project points in a pointcloud to a given camera image plane
func ProjectPointCloudToRGBPlane(pts *pointcloud.PointCloud, h, w int, params PinholeCameraIntrinsics, pixel2meter float64) (*pointcloud.PointCloud, error) {
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
				err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %s", j, i, pt.Position().Z, err)
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
