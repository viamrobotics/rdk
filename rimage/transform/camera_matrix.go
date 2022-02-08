package transform

import (
	"image"
	"image/color"
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
)

// AlignColorAndDepthImage takes in a RGB image and Depth map and aligns them according to the Aligner,
// returning a new ImageWithDepth.
func (dcie *DepthColorIntrinsicsExtrinsics) AlignColorAndDepthImage(c *rimage.Image, d *rimage.DepthMap) (*rimage.ImageWithDepth, error) {
	if c == nil {
		return nil, errors.New("no color image present to align")
	}
	if d == nil {
		return nil, errors.New("no depth image present to align")
	}
	return dcie.TransformDepthCoordToColorCoord(c, d)
}

// TransformDepthCoordToColorCoord changes the coordinate system of the depth map to be in same coordinate system
// as the color image.
func (dcie *DepthColorIntrinsicsExtrinsics) TransformDepthCoordToColorCoord(
	col *rimage.Image, dep *rimage.DepthMap) (*rimage.ImageWithDepth, error) {
	if col.Height() != dcie.ColorCamera.Height || col.Width() != dcie.ColorCamera.Width {
		return nil,
			errors.Errorf("camera matrices expected color image of (%#v,%#v), got (%#v, %#v)",
				dcie.ColorCamera.Width, dcie.ColorCamera.Height, col.Width(), col.Height())
	}
	if dep.Height() != dcie.DepthCamera.Height || dep.Width() != dcie.DepthCamera.Width {
		return nil,
			errors.Errorf("camera matrices expected depth image of (%#v,%#v), got (%#v, %#v)",
				dcie.DepthCamera.Width, dcie.DepthCamera.Height, dep.Width(), dep.Height())
	}
	outmap := rimage.NewEmptyDepthMap(dcie.ColorCamera.Width, dcie.ColorCamera.Height)
	for dy := 0; dy < dcie.DepthCamera.Height; dy++ {
		for dx := 0; dx < dcie.DepthCamera.Width; dx++ {
			dz := dep.GetDepth(dx, dy)
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
	return rimage.MakeImageWithDepth(col, outmap, true), nil
}

// ImagePointTo3DPoint takes in a image coordinate and returns the 3D point from the camera matrix.
func (dcie *DepthColorIntrinsicsExtrinsics) ImagePointTo3DPoint(point image.Point, depth rimage.Depth) (r3.Vector, error) {
	return intrinsics2DPtTo3DPt(point, depth, &dcie.ColorCamera)
}

// ImageWithDepthToPointCloud takes an ImageWithDepth and uses the camera parameters to project it to a pointcloud.
// Aligns it if it isn't already aligned.
func (dcie *DepthColorIntrinsicsExtrinsics) ImageWithDepthToPointCloud(ii *rimage.ImageWithDepth) (pointcloud.PointCloud, error) {
	var iwd *rimage.ImageWithDepth
	var err error
	// color and depth images need to already be aligned
	if ii.IsAligned() {
		iwd = ii
	} else {
		iwd, err = dcie.AlignColorAndDepthImage(ii.Color, ii.Depth)
		if err != nil {
			return nil, err
		}
	}
	return intrinsics2DTo3D(iwd, &dcie.ColorCamera)
}

// PointCloudToImageWithDepth takes a PointCloud with color info and returns an ImageWithDepth
// from the perspective of the color camera referenceframe.
func (dcie *DepthColorIntrinsicsExtrinsics) PointCloudToImageWithDepth(
	cloud pointcloud.PointCloud,
) (*rimage.ImageWithDepth, error) {
	return intrinsics3DTo2D(cloud, &dcie.ColorCamera)
}

// DepthPixelToColorPixel takes a pixel+depth (x,y, depth) from the depth camera and output is the coordinates
// of the color camera. Extrinsic matrices in meters, points are in mm, need to convert to m and then back.
func (dcie *DepthColorIntrinsicsExtrinsics) DepthPixelToColorPixel(dx, dy, dz float64) (float64, float64, float64) {
	m2mm := 1000.0
	x, y, z := dcie.DepthCamera.PixelToPoint(dx, dy, dz)
	x, y, z = x/m2mm, y/m2mm, z/m2mm
	x, y, z = dcie.ExtrinsicD2C.TransformPointToPoint(x, y, z)
	x, y, z = x*m2mm, y*m2mm, z*m2mm
	cx, cy := dcie.ColorCamera.PointToPixel(x, y, z)
	return cx, cy, z
}

// DepthMapToPointCloud converts a Depth Map to a PointCloud using the depth camera parameters.
func DepthMapToPointCloud(
	depthImage *rimage.DepthMap,
	pixel2meter float64,
	params *PinholeCameraIntrinsics,
	depthMin, depthMax rimage.Depth,
) (pointcloud.PointCloud, error) {
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
				xPoint /= pixel2meter
				yPoint /= pixel2meter
				z /= pixel2meter
				pt := pointcloud.NewBasicPoint(xPoint, yPoint, z)
				err := pcOut.Set(pt)
				if err != nil {
					err = errors.Wrapf(err, "error setting point (%v, %v, %v) in point cloud", xPoint, yPoint, z)
					return nil, err
				}
			}
		}
	}
	return pcOut, nil
}

// ApplyRigidBodyTransform projects a 3D point in a given camera image plane and return a
// new point cloud leaving the original unchanged.
func ApplyRigidBodyTransform(pts pointcloud.PointCloud, params *Extrinsics) (pointcloud.PointCloud, error) {
	transformedPoints := pointcloud.New()
	var err error
	pts.Iterate(func(pt pointcloud.Point) bool {
		x, y, z := params.TransformPointToPoint(pt.Position().X, pt.Position().Y, pt.Position().Z)
		var ptTransformed pointcloud.Point
		switch {
		case pt.HasColor():
			ptTransformed = pointcloud.NewColoredPoint(x, y, z, pt.Color().(color.NRGBA))
		case pt.HasValue():
			ptTransformed = pointcloud.NewValuePoint(x, y, z, pt.Value())
		default:
			ptTransformed = pointcloud.NewBasicPoint(x, y, z)
		}
		err = transformedPoints.Set(ptTransformed)
		if err != nil {
			err = errors.Wrapf(err, "error setting point (%v, %v, %v) in point cloud", x, y, z)
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return transformedPoints, nil
}

// ProjectPointCloudToRGBPlane projects points in a pointcloud to a given camera image plane.
func ProjectPointCloudToRGBPlane(
	pts pointcloud.PointCloud,
	h, w int,
	params PinholeCameraIntrinsics,
	pixel2meter float64,
) (pointcloud.PointCloud, error) {
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
				err = errors.Wrapf(err, "error setting point (%v, %v, %v) in point cloud", j, i, pt.Position().Z)
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

// TODO(louise): Add Depth Map dilation function as in the librealsense library
