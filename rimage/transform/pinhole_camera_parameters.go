//go:build !notc

package transform

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"os"

	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
)

// ErrNoIntrinsics is when a camera does not have intrinsics parameters or other parameters.
var ErrNoIntrinsics = errors.New("camera intrinsic parameters are not available")

// NewNoIntrinsicsError is used when the intriniscs are not defined.
func NewNoIntrinsicsError(msg string) error {
	return errors.Wrapf(ErrNoIntrinsics, msg)
}

// PinholeCameraModel is the model of a pinhole camera.
type PinholeCameraModel struct {
	*PinholeCameraIntrinsics `json:"intrinsic_parameters"`
	Distortion               Distorter `json:"distortion"`
}

// DistortionMap is a function that transforms the undistorted input points (u,v) to the distorted points (x,y)
// according to the model in PinholeCameraModel.Distortion.
func (params *PinholeCameraModel) DistortionMap() func(u, v float64) (float64, float64) {
	return func(u, v float64) (float64, float64) {
		x := (u - params.Ppx) / params.Fx
		y := (v - params.Ppy) / params.Fy
		x, y = params.Distortion.Transform(x, y)
		x = x*params.Fx + params.Ppx
		y = y*params.Fy + params.Ppy
		return x, y
	}
}

// UndistortImage takes an input image and creates a new image the same size with the same camera parameters
// as the original image, but undistorted according to the distortion model in PinholeCameraModel. A bilinear
// interpolation is used to interpolate values between image pixels.
// NOTE(bh): potentially a use case for generics
//
//nolint:dupl
func (params *PinholeCameraModel) UndistortImage(img *rimage.Image) (*rimage.Image, error) {
	if img == nil {
		return nil, errors.New("input image is nil")
	}
	// Check dimensions, they should be equal between the color image and what the intrinsics expect
	if params.Width != img.Width() || params.Height != img.Height() {
		return nil, errors.Errorf("img dimension and intrinsics don't match Image(%d,%d) != Intrinsics(%d,%d)",
			img.Width(), img.Height(), params.Width, params.Height)
	}
	undistortedImg := rimage.NewImage(params.Width, params.Height)
	distortionMap := params.DistortionMap()
	for v := 0; v < params.Height; v++ {
		for u := 0; u < params.Width; u++ {
			x, y := distortionMap(float64(u), float64(v))
			c := rimage.NearestNeighborColor(r2.Point{x, y}, img)
			if c != nil {
				undistortedImg.SetXY(u, v, *c)
			} else {
				undistortedImg.SetXY(u, v, rimage.Color(0))
			}
		}
	}
	return undistortedImg, nil
}

// UndistortDepthMap takes an input depth map and creates a new depth map the same size with the same camera parameters
// as the original depth map, but undistorted according to the distortion model in PinholeCameraModel. A nearest neighbor
// interpolation is used to interpolate values between depth pixels.
// NOTE(bh): potentially a use case for generics
//
//nolint:dupl
func (params *PinholeCameraModel) UndistortDepthMap(dm *rimage.DepthMap) (*rimage.DepthMap, error) {
	if dm == nil {
		return nil, errors.New("input DepthMap is nil")
	}
	// Check dimensions, they should be equal between the color image and what the intrinsics expect
	if params.Width != dm.Width() || params.Height != dm.Height() {
		return nil, errors.Errorf("img dimension and intrinsics don't match Image(%d,%d) != Intrinsics(%d,%d)",
			dm.Width(), dm.Height(), params.Width, params.Height)
	}
	undistortedDm := rimage.NewEmptyDepthMap(params.Width, params.Height)
	distortionMap := params.DistortionMap()
	for v := 0; v < params.Height; v++ {
		for u := 0; u < params.Width; u++ {
			x, y := distortionMap(float64(u), float64(v))
			d := rimage.NearestNeighborDepth(r2.Point{x, y}, dm)
			if d != nil {
				undistortedDm.Set(u, v, *d)
			} else {
				undistortedDm.Set(u, v, rimage.Depth(0))
			}
		}
	}
	return undistortedDm, nil
}

// PinholeCameraIntrinsics holds the parameters necessary to do a perspective projection of a 3D scene to the 2D plane.
type PinholeCameraIntrinsics struct {
	Width  int     `json:"width_px"`
	Height int     `json:"height_px"`
	Fx     float64 `json:"fx"`
	Fy     float64 `json:"fy"`
	Ppx    float64 `json:"ppx"`
	Ppy    float64 `json:"ppy"`
}

// CheckValid checks if the fields for PinholeCameraIntrinsics have valid inputs.
func (params *PinholeCameraIntrinsics) CheckValid() error {
	if params == nil {
		return NewNoIntrinsicsError("Intrinsics do not exist")
	}
	if params.Width == 0 || params.Height == 0 {
		return NewNoIntrinsicsError(fmt.Sprintf("Invalid size (%#v, %#v)", params.Width, params.Height))
	}
	if params.Fx <= 0 {
		return NewNoIntrinsicsError(fmt.Sprintf("Invalid focal length Fx = %#v", params.Fx))
	}
	if params.Fy <= 0 {
		return NewNoIntrinsicsError(fmt.Sprintf("Invalid focal length Fy = %#v", params.Fy))
	}
	if params.Ppx < 0 {
		return NewNoIntrinsicsError(fmt.Sprintf("Invalid principal X point Ppx = %#v", params.Ppx))
	}
	if params.Ppy < 0 {
		return NewNoIntrinsicsError(fmt.Sprintf("Invalid principal Y point Ppy = %#v", params.Ppy))
	}
	return nil
}

// NewPinholeCameraIntrinsicsFromJSONFile takes in a file path to a JSON and turns it into PinholeCameraIntrinsics.
func NewPinholeCameraIntrinsicsFromJSONFile(jsonPath string) (*PinholeCameraIntrinsics, error) {
	// open json file
	//nolint:gosec
	jsonFile, err := os.Open(jsonPath)
	if err != nil {
		err = errors.Wrap(err, "error opening JSON file")
		return nil, err
	}
	defer utils.UncheckedErrorFunc(jsonFile.Close)
	// read our opened jsonFile as a byte array.
	byteValue, err2 := io.ReadAll(jsonFile)
	if err2 != nil {
		err2 = errors.Wrap(err2, "error reading JSON data")
		return nil, err2
	}
	// Parse into map
	intrinsics := &PinholeCameraIntrinsics{}
	err = json.Unmarshal(byteValue, intrinsics)
	if err != nil {
		err = errors.Wrap(err, "error parsing JSON string")
		return nil, err
	}
	return intrinsics, nil
}

// PixelToPoint transforms a pixel with depth to a 3D point cloud.
// The intrinsics parameters should be the ones of the sensor used to obtain the image that
// contains the pixel.
func (params *PinholeCameraIntrinsics) PixelToPoint(x, y, z float64) (float64, float64, float64) {
	// TODO(louise): add unit test
	if params == nil {
		return float64(0), float64(0), float64(0)
	}
	xOverZ := (x - params.Ppx) / params.Fx
	yOverZ := (y - params.Ppy) / params.Fy
	// get x and y
	xm := xOverZ * z
	ym := yOverZ * z
	return xm, ym, z
}

// PointToPixel projects a 3D point to a pixel in an image plane.
// The intrinsics parameters should be the ones of the sensor we want to project to.
func (params *PinholeCameraIntrinsics) PointToPixel(x, y, z float64) (float64, float64) {
	// TODO(louise): add unit test
	if z != 0. {
		xPx := math.Round((x/z)*params.Fx + params.Ppx)
		yPx := math.Round((y/z)*params.Fy + params.Ppy)
		return xPx, yPx
	}
	// if depth is zero at this pixel, return negative coordinates so that the cropping to RGB bounds will filter it out
	return -1.0, -1.0
}

// ImagePointTo3DPoint takes in a image coordinate and returns the 3D point from the camera matrix.
func (params *PinholeCameraIntrinsics) ImagePointTo3DPoint(point image.Point, d rimage.Depth) (r3.Vector, error) {
	return intrinsics2DPtTo3DPt(point, d, params)
}

// RGBDToPointCloud takes an Image and Depth map and uses the camera parameters to project it to a pointcloud.
func (params *PinholeCameraIntrinsics) RGBDToPointCloud(
	img *rimage.Image, dm *rimage.DepthMap,
	crop ...image.Rectangle,
) (pointcloud.PointCloud, error) {
	var rect *image.Rectangle
	if len(crop) > 1 {
		return nil, errors.Errorf("cannot have more than one cropping rectangle, got %v", crop)
	}
	if len(crop) == 1 {
		rect = &crop[0]
	}
	return intrinsics2DTo3D(img, dm, params, rect)
}

// PointCloudToRGBD takes a PointCloud with color info and returns an Image and DepthMap from the
// perspective of the camera referenceframe.
func (params *PinholeCameraIntrinsics) PointCloudToRGBD(
	cloud pointcloud.PointCloud,
) (*rimage.Image, *rimage.DepthMap, error) {
	if params == nil {
		return nil, nil, nil
	}
	return intrinsics3DTo2D(cloud, params)
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
	pts.Iterate(0, 0, func(pt r3.Vector, d pointcloud.Data) bool {
		j, i := params.PointToPixel(pt.X, pt.Y, pt.Z)
		j = math.Round(j)
		i = math.Round(i)
		// if point has color is inside the RGB image bounds, add it to the new pointcloud
		if j >= 0 && j < float64(w) && i >= 0 && i < float64(h) && d != nil && d.HasColor() {
			pt2d := pointcloud.NewVector(j, i, pt.Z)
			err = coordinates.Set(pt2d, d)
			if err != nil {
				err = errors.Wrapf(err, "error setting point (%v, %v, %v) in point cloud", j, i, pt.Z)
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

// GetCameraMatrix creates a new camera matrix and returns it.
// Camera matrix:
// [[fx 0 ppx],
//
//	[0 fy ppy],
//	[0 0  1]]
func (params *PinholeCameraIntrinsics) GetCameraMatrix() *mat.Dense {
	if params == nil {
		return nil
	}
	cameraMatrix := mat.NewDense(3, 3, nil)
	cameraMatrix.Set(0, 0, params.Fx)
	cameraMatrix.Set(1, 1, params.Fy)
	cameraMatrix.Set(0, 2, params.Ppx)
	cameraMatrix.Set(1, 2, params.Ppy)
	cameraMatrix.Set(2, 2, 1)
	return cameraMatrix
}

// intrinsics2DPtTo3DPt takes in a image coordinate and returns the 3D point using the camera's intrinsic matrix.
func intrinsics2DPtTo3DPt(pt image.Point, d rimage.Depth, pci *PinholeCameraIntrinsics) (r3.Vector, error) {
	px, py, pz := pci.PixelToPoint(float64(pt.X), float64(pt.Y), float64(d))
	return r3.Vector{px, py, pz}, nil
}

// intrinsics3DTo2D uses the camera's intrinsic matrix to project the 3D pointcloud to a 2D image and depth map.
func intrinsics3DTo2D(cloud pointcloud.PointCloud, pci *PinholeCameraIntrinsics) (*rimage.Image, *rimage.DepthMap, error) {
	// Needs to be a pointcloud with color
	if !cloud.MetaData().HasColor {
		return nil, nil, errors.New("pointcloud has no color information, cannot create an image with depth")
	}
	// Image and DepthMap will be in the camera frame of the camera specified by PinholeCameraIntrinsics.
	// Points outside of the frame will be discarded.
	// Assumption is that points in pointcloud are in mm.
	width, height := pci.Width, pci.Height
	color := rimage.NewImage(width, height)
	depth := rimage.NewEmptyDepthMap(width, height)
	cloud.Iterate(0, 0, func(pt r3.Vector, d pointcloud.Data) bool {
		j, i := pci.PointToPixel(pt.X, pt.Y, pt.Z)
		x, y := int(math.Round(j)), int(math.Round(i))
		z := int(pt.Z)
		// if point has color and is inside the image bounds, add it to the images
		if x >= 0 && x < width && y >= 0 && y < height && d != nil && d.HasColor() {
			r, g, b := d.RGB255()
			color.Set(image.Point{x, y}, rimage.NewColor(r, g, b))
			depth.Set(x, y, rimage.Depth(z))
		}
		return true
	})
	return color, depth, nil
}

// intrinsics2DTo3D uses the camera's intrinsic matrix to project the 2D image and depth map to a 3D point cloud.
func intrinsics2DTo3D(img *rimage.Image, dm *rimage.DepthMap, pci *PinholeCameraIntrinsics, crop *image.Rectangle,
) (pointcloud.PointCloud, error) {
	if img == nil {
		return nil, errors.New("no rgb channel. Cannot project to Pointcloud")
	}
	if dm == nil {
		return nil, errors.New("no depth channel. Cannot project to Pointcloud")
	}
	// Check dimensions, they should be equal between the color and depth frame
	if img.Bounds() != dm.Bounds() {
		return nil, errors.Errorf("depth map and color dimensions don't match Depth(%d,%d) != Color(%d,%d)",
			dm.Width(), dm.Height(), img.Width(), img.Height())
	}
	startX, startY := 0, 0
	endX, endY := img.Width(), img.Height()
	// if optional crop rectangle is provided, use intersections of rectangle and image window and iterate through it
	if crop != nil {
		newBounds := crop.Intersect(img.Bounds())
		startX, startY = newBounds.Min.X, newBounds.Min.Y
		endX, endY = newBounds.Max.X, newBounds.Max.Y
	}
	pc := pointcloud.NewWithPrealloc((endY - startY) * (endX - startX))

	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			px, py, pz := pci.PixelToPoint(float64(x), float64(y), float64(dm.GetDepth(x, y)))
			r, g, b := img.GetXY(x, y).RGB255()
			err := pc.Set(pointcloud.NewVector(px, py, pz), pointcloud.NewColoredData(color.NRGBA{r, g, b, 255}))
			if err != nil {
				return nil, err
			}
		}
	}
	return pc, nil
}
