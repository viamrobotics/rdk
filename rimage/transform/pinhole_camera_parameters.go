package transform

import (
	"encoding/json"
	"io/ioutil"
	"math"
	"os"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/config"
)

// DistortionModel TODO
type DistortionModel struct {
	RadialK1     float64 `json:"rk1"`
	RadialK2     float64 `json:"rk2"`
	RadialK3     float64 `json:"rk3"`
	TangentialP1 float64 `json:"tp1"`
	TangentialP2 float64 `json:"tp2"`
}

// PinholeCameraIntrinsics TODO
type PinholeCameraIntrinsics struct {
	Width      int             `json:"width"`
	Height     int             `json:"height"`
	Fx         float64         `json:"fx"`
	Fy         float64         `json:"fy"`
	Ppx        float64         `json:"ppx"`
	Ppy        float64         `json:"ppy"`
	Distortion DistortionModel `json:"distortion"`
}

// Extrinsics TODO
type Extrinsics struct {
	RotationMatrix    []float64 `json:"rotation"`
	TranslationVector []float64 `json:"translation"`
}

// DepthColorIntrinsicsExtrinsics TODO
type DepthColorIntrinsicsExtrinsics struct {
	ColorCamera  PinholeCameraIntrinsics `json:"color"`
	DepthCamera  PinholeCameraIntrinsics `json:"depth"`
	ExtrinsicD2C Extrinsics              `json:"extrinsicsDepthToColor"`
}

// CheckValid TODO
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

// NewEmptyDepthColorIntrinsicsExtrinsics TODO
func NewEmptyDepthColorIntrinsicsExtrinsics() *DepthColorIntrinsicsExtrinsics {
	return &DepthColorIntrinsicsExtrinsics{
		ColorCamera:  PinholeCameraIntrinsics{0, 0, 0, 0, 0, 0, DistortionModel{0, 0, 0, 0, 0}},
		DepthCamera:  PinholeCameraIntrinsics{0, 0, 0, 0, 0, 0, DistortionModel{0, 0, 0, 0, 0}},
		ExtrinsicD2C: Extrinsics{[]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}, []float64{0, 0, 0}},
	}
}

// NewDepthColorIntrinsicsExtrinsics TODO
func NewDepthColorIntrinsicsExtrinsics(attrs config.AttributeMap) (*DepthColorIntrinsicsExtrinsics, error) {
	var matrices *DepthColorIntrinsicsExtrinsics

	if attrs.Has("matrices") {
		matrices = attrs["matrices"].(*DepthColorIntrinsicsExtrinsics)
	} else {
		return nil, errors.New("no camera config")
	}
	return matrices, nil
}

// NewDepthColorIntrinsicsExtrinsicsFromBytes TODO
func NewDepthColorIntrinsicsExtrinsicsFromBytes(byteJSON []byte) (*DepthColorIntrinsicsExtrinsics, error) {
	intrinsics := NewEmptyDepthColorIntrinsicsExtrinsics()
	// Parse into map
	err := json.Unmarshal(byteJSON, intrinsics)
	if err != nil {
		err = errors.Errorf("error parsing byte array - %w", err)
		return nil, err
	}
	return intrinsics, nil
}

// NewDepthColorIntrinsicsExtrinsicsFromJSONFile TODO
func NewDepthColorIntrinsicsExtrinsicsFromJSONFile(jsonPath string) (*DepthColorIntrinsicsExtrinsics, error) {
	// open json file
	jsonFile, err := os.Open(jsonPath)
	if err != nil {
		err = errors.Errorf("error opening JSON file - %w", err)
		return nil, err
	}
	defer utils.UncheckedErrorFunc(jsonFile.Close)
	// read our opened jsonFile as a byte array.
	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		err = errors.Errorf("error reading JSON data - %w", err)
		return nil, err
	}
	return NewDepthColorIntrinsicsExtrinsicsFromBytes(byteValue)
}

// NewPinholeCameraIntrinsicsFromJSONFile TODO
func NewPinholeCameraIntrinsicsFromJSONFile(jsonPath, cameraName string) (*PinholeCameraIntrinsics, error) {
	intrinsics := NewEmptyDepthColorIntrinsicsExtrinsics()
	// open json file
	jsonFile, err := os.Open(jsonPath)
	if err != nil {
		err = errors.Errorf("error opening JSON file - %w", err)
		return nil, err
	}
	defer utils.UncheckedErrorFunc(jsonFile.Close)
	// read our opened jsonFile as a byte array.
	byteValue, err2 := ioutil.ReadAll(jsonFile)
	if err2 != nil {
		err2 = errors.Errorf("error reading JSON data - %w", err2)
		return nil, err2
	}
	// Parse into map
	err = json.Unmarshal(byteValue, intrinsics)
	if err != nil {
		err = errors.Errorf("error parsing JSON string - %w", err)
		return nil, err
	}
	if cameraName == "depth" {
		return &intrinsics.DepthCamera, nil
	}
	return &intrinsics.ColorCamera, nil
}

// PixelToPoint transforms a pixel with depth to a 3D point cloud.
// The intrinsics parameters should be the ones of the sensor used to obtain the image that
// contains the pixel.
func (params *PinholeCameraIntrinsics) PixelToPoint(x, y, z float64) (float64, float64, float64) {
	//TODO(louise): add unit test
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
	//TODO(louise): add unit test
	if z != 0. {
		xPx := math.Round((x/z)*params.Fx + params.Ppx)
		yPx := math.Round((y/z)*params.Fy + params.Ppy)
		return xPx, yPx
	}
	// if depth is zero at this pixel, return negative coordinates so that the cropping to RGB bounds will filter it out
	return -1.0, -1.0
}

// TransformPointToPoint applies a rigid body transform between two cameras to a 3D point.
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
