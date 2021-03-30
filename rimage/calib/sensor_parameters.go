package calib

import (
	"encoding/json"
	"fmt"
	"image"
	"io/ioutil"
	"math"
	"os"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/pointcloud"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/utils"

	"github.com/edaniels/golog"
)

type DistortionModel struct {
	RadialK1     float64 `json:"rk1"`
	RadialK2     float64 `json:"rk2"`
	RadialK3     float64 `json:"rk3"`
	TangentialP1 float64 `json:"tp1"`
	TangentialP2 float64 `json:"tp2"`
}

type PinholeCameraIntrinsics struct {
	Width      int             `json:"width"`
	Height     int             `json:"height"`
	Fx         float64         `json:"fx"`
	Fy         float64         `json:"fy"`
	Ppx        float64         `json:"ppx"`
	Ppy        float64         `json:"ppy"`
	Distortion DistortionModel `json:"distortion"`
}

type Extrinsics struct {
	RotationMatrix    []float64 `json:"rotation"`
	TranslationVector []float64 `json:"translation"`
}

type DepthColorIntrinsicsExtrinsics struct {
	ColorCamera  PinholeCameraIntrinsics `json:"color"`
	DepthCamera  PinholeCameraIntrinsics `json:"depth"`
	ExtrinsicD2C Extrinsics              `json:"extrinsicsDepthToColor"`
}

func (dcie *DepthColorIntrinsicsExtrinsics) CheckValid() error {
	if dcie == nil {
		return fmt.Errorf("pointer to DepthColorIntrinsicsExtrinsics is nil")
	}
	if dcie.ColorCamera.Width == 0 || dcie.ColorCamera.Height == 0 {
		return fmt.Errorf("invalid ColorSize (%#v, %#v)", dcie.ColorCamera.Width, dcie.ColorCamera.Height)
	}
	if dcie.DepthCamera.Width == 0 || dcie.DepthCamera.Height == 0 {
		return fmt.Errorf("invalid DepthSize (%#v, %#v)", dcie.DepthCamera.Width, dcie.DepthCamera.Height)
	}
	return nil
}

func (dcie *DepthColorIntrinsicsExtrinsics) ToAlignedImageWithDepth(ii *rimage.ImageWithDepth, logger golog.Logger) (*rimage.ImageWithDepth, error) {
	newImgWithDepth, err := dcie.TransformDepthCoordToColorCoord(ii)
	if err != nil {
		return nil, err
	}
	return newImgWithDepth, nil
}

func (dcie *DepthColorIntrinsicsExtrinsics) ToPointCloudWithColor(ii *rimage.ImageWithDepth, logger golog.Logger) (*pointcloud.PointCloud, error) {
	return nil, fmt.Errorf("method ToPointCloudWithColor not implemented for DepthColorIntrinsicsExtrinsics")
}

func NewEmptyDepthColorIntrinsicsExtrinsics() *DepthColorIntrinsicsExtrinsics {
	return &DepthColorIntrinsicsExtrinsics{
		ColorCamera:  PinholeCameraIntrinsics{0, 0, 0, 0, 0, 0, DistortionModel{0, 0, 0, 0, 0}},
		DepthCamera:  PinholeCameraIntrinsics{0, 0, 0, 0, 0, 0, DistortionModel{0, 0, 0, 0, 0}},
		ExtrinsicD2C: Extrinsics{[]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}, []float64{0, 0, 0}},
	}
}

func NewDepthColorIntrinsicsExtrinsics(attrs api.AttributeMap, logger golog.Logger) (*DepthColorIntrinsicsExtrinsics, error) {
	var matrices *DepthColorIntrinsicsExtrinsics

	if attrs.Has("matrices") {
		matrices = attrs["matrices"].(*DepthColorIntrinsicsExtrinsics)
	} else {
		return nil, fmt.Errorf("no alignment config")
	}
	return matrices, nil
}

func NewDepthColorIntrinsicsExtrinsicsFromJSONFile(jsonPath string) (*DepthColorIntrinsicsExtrinsics, error) {
	intrinsics := NewEmptyDepthColorIntrinsicsExtrinsics()
	// open json file
	jsonFile, err := os.Open(jsonPath)
	if err != nil {
		err = fmt.Errorf("error opening JSON file - %s", err)
		return intrinsics, err
	}
	defer jsonFile.Close()
	// read our opened jsonFile as a byte array.
	byteValue, err2 := ioutil.ReadAll(jsonFile)
	if err2 != nil {
		err2 = fmt.Errorf("error reading JSON data - %s", err2)
		return intrinsics, err2
	}
	// Parse into map
	err = json.Unmarshal(byteValue, intrinsics)
	if err != nil {
		err = fmt.Errorf("error parsing JSON string - %s", err)
		return intrinsics, err
	}
	return intrinsics, nil
}

func NewPinholeCameraIntrinsicsFromJSONFile(jsonPath, cameraName string) (*PinholeCameraIntrinsics, error) {
	intrinsics := NewEmptyDepthColorIntrinsicsExtrinsics()
	// open json file
	jsonFile, err := os.Open(jsonPath)
	if err != nil {
		err = fmt.Errorf("error opening JSON file - %s", err)
		return nil, err
	}
	defer jsonFile.Close()
	// read our opened jsonFile as a byte array.
	byteValue, err2 := ioutil.ReadAll(jsonFile)
	if err2 != nil {
		err2 = fmt.Errorf("error reading JSON data - %s", err2)
		return nil, err2
	}
	// Parse into map
	err = json.Unmarshal(byteValue, intrinsics)
	if err != nil {
		err = fmt.Errorf("error parsing JSON string - %s", err)
		return nil, err
	}
	if cameraName == "depth" {
		return &intrinsics.DepthCamera, nil
	}
	return &intrinsics.ColorCamera, nil
}

// Function to transform a pixel with depth to a 3D point cloud
// the intrinsics parameters should be the ones of the sensor used to obtain the image that contains the pixel
func (params *PinholeCameraIntrinsics) PixelToPoint(x, y int, z float64) (float64, float64, float64) {
	//TODO(louise): add unit test
	xOverZ := (float64(x) - params.Ppx) / params.Fx
	yOverZ := (float64(y) - params.Ppy) / params.Fy
	// get x and y
	xm := xOverZ * z
	ym := yOverZ * z
	return xm, ym, z
}

// Function to project a 3D point to a pixel in an image plane
// the intrinsics parameters should be the ones of the sensor we want to project to
func (params *PinholeCameraIntrinsics) PointToPixel(x, y, z float64) (float64, float64) {
	//TODO(louise): add unit test
	if z != 0. {
		xPx := math.Round((x/z)*params.Fx + params.Ppx)
		yPx := math.Round((y/z)*params.Fy + params.Ppy)
		return xPx, yPx
	}
	// if depth is zero at this pixel, return negative coordinates so that the cropping to RGB bounds will filter it out
	return -1.0, -1.0
}

// Function to apply a rigid body transform between two cameras to a 3D point
func (params *Extrinsics) TransformPointToPoint(x, y, z float64) (float64, float64, float64) {
	rotationMatrix := params.RotationMatrix
	translationVector := params.TranslationVector
	n := len(rotationMatrix)
	if n != 9 {
		panic("Rotation Matrix to transform point cloud should be a 3x3 matrix")
	}
	xTransformed := rotationMatrix[0]*x + rotationMatrix[1]*y + rotationMatrix[2]*z + translationVector[0]
	yTransformed := rotationMatrix[3]*x + rotationMatrix[4]*y + rotationMatrix[5]*z + translationVector[1]
	zTransformed := rotationMatrix[6]*x + rotationMatrix[7]*y + rotationMatrix[8]*z + translationVector[2]

	return xTransformed, yTransformed, zTransformed
}

// Function input is a pixel+depth (x,y, depth) from the depth camera and output is the coordinates of the color camera
func (dcie *DepthColorIntrinsicsExtrinsics) DepthPixelToColorPixel(dx, dy int, dz float64) (int, int, float64) {
	x, y, z := dcie.DepthCamera.PixelToPoint(dx, dy, dz)
	x, y, z = dcie.ExtrinsicD2C.TransformPointToPoint(x, y, z)
	cx, cy := dcie.ColorCamera.PointToPixel(x, y, z)
	return int(cx), int(cy), z
}

// change coordinate system of depth map to be in same coordinate system as color image
// then crop both images
// TODO(bijan): make this use matrix multiplication rather than loops
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
	for x := 0; x < dcie.DepthCamera.Width; x++ {
		for y := 0; y < dcie.DepthCamera.Height; y++ {
			z := inmap.GetDepth(x, y)
			if z == 0 {
				continue
			}
			cx, cy, cz := dcie.DepthPixelToColorPixel(x, y, float64(z))
			if cx < 0 || cy < 0 || cx > dcie.ColorCamera.Width-1 || cy > dcie.ColorCamera.Height-1 {
				continue
			}
			xMin, yMin = utils.MinInt(xMin, cx), utils.MinInt(yMin, cy)
			xMax, yMax = utils.MaxInt(xMax, cx), utils.MaxInt(yMax, cy)
			outmap.Set(cx, cy, rimage.Depth(cz))
		}
	}
	crop := image.Rect(xMin, yMin, xMax, yMax)
	outmap = outmap.SubImage(crop)
	outcol := img.Color.SubImage(crop)
	return &rimage.ImageWithDepth{&outcol, &outmap}, nil
}
