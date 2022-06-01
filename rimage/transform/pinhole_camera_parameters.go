package transform

import (
	"encoding/json"
	"image"
	"image/color"
	"io/ioutil"
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

// DistortionModel TODO.
type DistortionModel struct {
	RadialK1     float64 `json:"rk1"`
	RadialK2     float64 `json:"rk2"`
	RadialK3     float64 `json:"rk3"`
	TangentialP1 float64 `json:"tp1"`
	TangentialP2 float64 `json:"tp2"`
}

// Transform distorts the input points x,y according to a modified Brown-Conrady model as described by OpenCV
// https://docs.opencv.org/3.4/da/d54/group__imgproc__transform.html#ga7dfb72c9cf9780a347fbe3d1c47e5d5a
func (dm *DistortionModel) Transform(x, y float64) (float64, float64) {
	r2 := x*x + y*y
	radDist := (1. + dm.RadialK1*r2 + dm.RadialK2*r2*r2 + dm.RadialK3*r2*r2*r2)
	radDistX := x * radDist
	radDistY := y * radDist
	tanDistX := 2.*dm.TangentialP1*x*y + dm.TangentialP2*(r2+2.*x*x)
	tanDistY := 2.*dm.TangentialP2*x*y + dm.TangentialP1*(r2+2.*y*y)
	resX := radDistX + tanDistX
	resY := radDistY + tanDistY
	return resX, resY
}

// PinholeCameraIntrinsics TODO.
type PinholeCameraIntrinsics struct {
	Width      int             `json:"width"`
	Height     int             `json:"height"`
	Fx         float64         `json:"fx"`
	Fy         float64         `json:"fy"`
	Ppx        float64         `json:"ppx"`
	Ppy        float64         `json:"ppy"`
	Distortion DistortionModel `json:"distortion"`
}

// Extrinsics TODO.
type Extrinsics struct {
	RotationMatrix    []float64 `json:"rotation"`
	TranslationVector []float64 `json:"translation"`
}

// DepthColorIntrinsicsExtrinsics TODO.
type DepthColorIntrinsicsExtrinsics struct {
	ColorCamera  PinholeCameraIntrinsics `json:"color"`
	DepthCamera  PinholeCameraIntrinsics `json:"depth"`
	ExtrinsicD2C Extrinsics              `json:"extrinsics_depth_to_color"`
}

// CheckValid checks if the fields for PinholeCameraIntrinsics have valid inputs.
func (params *PinholeCameraIntrinsics) CheckValid() error {
	if params == nil {
		return errors.New("pointer to PinholeCameraIntrinsics is nil")
	}
	if params.Width == 0 || params.Height == 0 {
		return errors.Errorf("invalid size (%#v, %#v)", params.Width, params.Height)
	}
	return nil
}

// CheckValid TODO.
func (dcie *DepthColorIntrinsicsExtrinsics) CheckValid() error {
	if dcie == nil {
		return errors.New("pointer to DepthColorIntrinsicsExtrinsics is nil")
	}
	if dcie.ColorCamera.Width == 0 || dcie.ColorCamera.Height == 0 {
		return errors.Errorf("invalid ColorSize (%#v, %#v)", dcie.ColorCamera.Width, dcie.ColorCamera.Height)
	}
	if dcie.DepthCamera.Width == 0 || dcie.DepthCamera.Height == 0 {
		return errors.Errorf("invalid DepthSize (%#v, %#v)", dcie.DepthCamera.Width, dcie.DepthCamera.Height)
	}
	return nil
}

// NewEmptyDepthColorIntrinsicsExtrinsics TODO.
func NewEmptyDepthColorIntrinsicsExtrinsics() *DepthColorIntrinsicsExtrinsics {
	return &DepthColorIntrinsicsExtrinsics{
		ColorCamera:  PinholeCameraIntrinsics{0, 0, 0, 0, 0, 0, DistortionModel{0, 0, 0, 0, 0}},
		DepthCamera:  PinholeCameraIntrinsics{0, 0, 0, 0, 0, 0, DistortionModel{0, 0, 0, 0, 0}},
		ExtrinsicD2C: Extrinsics{[]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}, []float64{0, 0, 0}},
	}
}

// NewDepthColorIntrinsicsExtrinsicsFromBytes TODO.
func NewDepthColorIntrinsicsExtrinsicsFromBytes(byteJSON []byte) (*DepthColorIntrinsicsExtrinsics, error) {
	intrinsics := NewEmptyDepthColorIntrinsicsExtrinsics()
	// Parse into map
	err := json.Unmarshal(byteJSON, intrinsics)
	if err != nil {
		err = errors.Wrap(err, "error parsing byte array")
		return nil, err
	}
	return intrinsics, nil
}

// NewDepthColorIntrinsicsExtrinsicsFromJSONFile TODO.
func NewDepthColorIntrinsicsExtrinsicsFromJSONFile(jsonPath string) (*DepthColorIntrinsicsExtrinsics, error) {
	// open json file
	//nolint:gosec
	jsonFile, err := os.Open(jsonPath)
	if err != nil {
		err = errors.Wrap(err, "error opening JSON file")
		return nil, err
	}
	defer utils.UncheckedErrorFunc(jsonFile.Close)
	// read our opened jsonFile as a byte array.
	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		err = errors.Wrap(err, "error reading JSON data")
		return nil, err
	}
	return NewDepthColorIntrinsicsExtrinsicsFromBytes(byteValue)
}

// NewPinholeCameraIntrinsicsFromJSONFile TODO.
func NewPinholeCameraIntrinsicsFromJSONFile(jsonPath, cameraName string) (*PinholeCameraIntrinsics, error) {
	intrinsics := NewEmptyDepthColorIntrinsicsExtrinsics()
	// open json file
	//nolint:gosec
	jsonFile, err := os.Open(jsonPath)
	if err != nil {
		err = errors.Wrap(err, "error opening JSON file")
		return nil, err
	}
	defer utils.UncheckedErrorFunc(jsonFile.Close)
	// read our opened jsonFile as a byte array.
	byteValue, err2 := ioutil.ReadAll(jsonFile)
	if err2 != nil {
		err2 = errors.Wrap(err2, "error reading JSON data")
		return nil, err2
	}
	// Parse into map
	err = json.Unmarshal(byteValue, intrinsics)
	if err != nil {
		err = errors.Wrap(err, "error parsing JSON string")
		return nil, err
	}
	if cameraName == "depth" {
		return &intrinsics.DepthCamera, nil
	}
	return &intrinsics.ColorCamera, nil
}

// DistortionMap is a function that transforms the undistorted input points (u,v) to the distorted points (x,y)
// according to the model in PinholeCameraIntrinsics.Distortion.
func (params *PinholeCameraIntrinsics) DistortionMap() func(u, v float64) (float64, float64) {
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
// as the original image, but undistorted according to the distortion model in PinholeCameraIntrinsics. A bilinear
// interpolation is used to interpolate values between image pixels.
// NOTE(bh): potentially a use case for generics
//nolint:dupl
func (params *PinholeCameraIntrinsics) UndistortImage(img *rimage.Image) (*rimage.Image, error) {
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
// as the original depth map, but undistorted according to the distortion model in PinholeCameraIntrinsics. A nearest neighbor
// interpolation is used to interpolate values between depth pixels.
// NOTE(bh): potentially a use case for generics
//nolint:dupl
func (params *PinholeCameraIntrinsics) UndistortDepthMap(dm *rimage.DepthMap) (*rimage.DepthMap, error) {
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

// PixelToPoint transforms a pixel with depth to a 3D point cloud.
// The intrinsics parameters should be the ones of the sensor used to obtain the image that
// contains the pixel.
func (params *PinholeCameraIntrinsics) PixelToPoint(x, y, z float64) (float64, float64, float64) {
	// TODO(louise): add unit test
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

// ImageWithDepthToPointCloud takes an ImageWithDepth and uses the camera parameters to project it to a pointcloud.
func (params *PinholeCameraIntrinsics) ImageWithDepthToPointCloud(
	ii *rimage.ImageWithDepth,
	crop ...image.Rectangle,
) (pointcloud.PointCloud, error) {
	var rect *image.Rectangle
	if len(crop) > 1 {
		return nil, errors.Errorf("cannot have more than one cropping rectangle, got %v", crop)
	}
	if len(crop) == 1 {
		rect = &crop[0]
	}
	return intrinsics2DTo3D(ii, params, rect)
}

// PointCloudToImageWithDepth takes a PointCloud with color info and returns an ImageWithDepth from the
// perspective of the camera referenceframe.
func (params *PinholeCameraIntrinsics) PointCloudToImageWithDepth(
	cloud pointcloud.PointCloud,
) (*rimage.ImageWithDepth, error) {
	return intrinsics3DTo2D(cloud, params)
}

// TransformPointToPoint applies a rigid body transform between two cameras to a 3D point.
func (params *Extrinsics) TransformPointToPoint(x, y, z float64) (float64, float64, float64) {
	rotationMatrix := params.RotationMatrix
	translationVector := params.TranslationVector
	if len(rotationMatrix) != 9 {
		panic("Rotation Matrix to transform point cloud should be a 3x3 matrix")
	}
	xTransformed := rotationMatrix[0]*x + rotationMatrix[1]*y + rotationMatrix[2]*z + translationVector[0]
	yTransformed := rotationMatrix[3]*x + rotationMatrix[4]*y + rotationMatrix[5]*z + translationVector[1]
	zTransformed := rotationMatrix[6]*x + rotationMatrix[7]*y + rotationMatrix[8]*z + translationVector[2]

	return xTransformed, yTransformed, zTransformed
}

// intrinsics2DPtTo3DPt takes in a image coordinate and returns the 3D point using the camera's intrinsic matrix.
func intrinsics2DPtTo3DPt(pt image.Point, d rimage.Depth, pci *PinholeCameraIntrinsics) (r3.Vector, error) {
	px, py, pz := pci.PixelToPoint(float64(pt.X), float64(pt.Y), float64(d))
	return r3.Vector{px, py, pz}, nil
}

// intrinsics3DTo2D uses the camera's intrinsic matrix to project the 3D pointcloud to a 2D image with depth.
func intrinsics3DTo2D(cloud pointcloud.PointCloud, pci *PinholeCameraIntrinsics) (*rimage.ImageWithDepth, error) {
	// Needs to be a pointcloud with color
	if !cloud.MetaData().HasColor {
		return nil, errors.New("pointcloud has no color information, cannot create an image with depth")
	}
	// ImageWithDepth will be in the camera frame of the camera specified by PinholeCameraIntrinsics.
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
	return rimage.MakeImageWithDepth(color, depth, true), nil
}

// intrinsics2DTo3D uses the camera's intrinsic matrix to project the 2D image with depth to a 3D point cloud.
func intrinsics2DTo3D(iwd *rimage.ImageWithDepth, pci *PinholeCameraIntrinsics, crop *image.Rectangle) (pointcloud.PointCloud, error) {
	if iwd.Depth == nil {
		return nil, errors.New("image with depth has no depth channel. Cannot project to Pointcloud")
	}
	if !iwd.IsAligned() {
		return nil, errors.New("color and depth are not aligned. Cannot project to Pointcloud")
	}
	// Check dimensions, they should be equal between the color and depth frame
	if iwd.Depth.Width() != iwd.Color.Width() || iwd.Depth.Height() != iwd.Color.Height() {
		return nil, errors.Errorf("depth map and color dimensions don't match Depth(%d,%d) != Color(%d,%d)",
			iwd.Depth.Width(), iwd.Depth.Height(), iwd.Color.Width(), iwd.Color.Height())
	}
	startX, startY := 0, 0
	endX, endY := iwd.Width(), iwd.Height()
	// if optional crop rectangle is provided, use intersections of rectangle and image window and iterate through it
	if crop != nil {
		newBounds := crop.Intersect(iwd.Bounds())
		startX, startY = newBounds.Min.X, newBounds.Min.Y
		endX, endY = newBounds.Max.X, newBounds.Max.Y
	}
	pc := pointcloud.NewWithPrealloc((endY - startY) * (endX - startX))

	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			px, py, pz := pci.PixelToPoint(float64(x), float64(y), float64(iwd.Depth.GetDepth(x, y)))
			r, g, b := iwd.Color.GetXY(x, y).RGB255()
			err := pc.Set(pointcloud.NewVector(px, py, pz), pointcloud.NewColoredData(color.NRGBA{r, g, b, 255}))
			if err != nil {
				return nil, err
			}
		}
	}
	return pc, nil
}

// GetCameraMatrix creates a new camera matrix and returns it.
// Camera matrix:
// [[fx 0 ppx],
//  [0 fy ppy],
//  [0 0  1]]
func (params *PinholeCameraIntrinsics) GetCameraMatrix() *mat.Dense {
	cameraMatrix := mat.NewDense(3, 3, nil)
	cameraMatrix.Set(0, 0, params.Fx)
	cameraMatrix.Set(1, 1, params.Fy)
	cameraMatrix.Set(0, 2, params.Ppx)
	cameraMatrix.Set(1, 2, params.Ppy)
	cameraMatrix.Set(2, 2, 1)
	return cameraMatrix
}
