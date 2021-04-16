package calib

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
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

func (dcie *DepthColorIntrinsicsExtrinsics) ToAlignedImageWithDepth(ii *rimage.ImageWithDepth) (*rimage.ImageWithDepth, error) {
	if ii.IsAligned() {
		return ii, nil
	}
	newImgWithDepth, err := dcie.TransformDepthCoordToColorCoord(ii)
	if err != nil {
		return nil, err
	}
	return newImgWithDepth, nil
}

func (dcie *DepthColorIntrinsicsExtrinsics) ToPointCloudWithColor(ii *rimage.ImageWithDepth) (*pointcloud.PointCloud, error) {
	var newImgWithDepth *rimage.ImageWithDepth
	var err error
	if ii.IsAligned() {
		newImgWithDepth = ii
	} else {
		newImgWithDepth, err = dcie.TransformDepthCoordToColorCoord(ii)
		if err != nil {
			return nil, err
		}
	}
	// All points now in Color frame
	pc, err := dcie.AlignedImageToPointCloud(newImgWithDepth)
	if err != nil {
		return nil, err
	}
	return pc, nil
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
func (params *PinholeCameraIntrinsics) PixelToPoint(x, y, z float64) (float64, float64, float64) {
	//TODO(louise): add unit test
	xOverZ := (x - params.Ppx) / params.Fx
	yOverZ := (y - params.Ppy) / params.Fy
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
// Extrinsic matrices in meters, points are in mm, need to convert to m and then back
func (dcie *DepthColorIntrinsicsExtrinsics) DepthPixelToColorPixel(dx, dy, dz float64) (float64, float64, float64) {
	m2mm := 1000.0
	x, y, z := dcie.DepthCamera.PixelToPoint(dx, dy, dz)
	x, y, z = x/m2mm, y/m2mm, z/m2mm
	x, y, z = dcie.ExtrinsicD2C.TransformPointToPoint(x, y, z)
	x, y, z = x*m2mm, y*m2mm, z*m2mm
	cx, cy := dcie.ColorCamera.PointToPixel(x, y, z)
	return cx, cy, z
}

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
