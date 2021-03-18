package calibration

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"reflect"

	"github.com/golang/geo/r3"
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

func NewDistortionModel() *DistortionModel {
	return &DistortionModel{
		RadialK1:     0,
		RadialK2:     0,
		RadialK3:     0,
		TangentialP1: 0,
		TangentialP2: 0,
	}
}

func NewExtrinsics() *Extrinsics {
	return &Extrinsics{
		RotationMatrix:    nil,
		TranslationVector: nil,
	}
}

func NewPinholeCameraIntrinsics() *PinholeCameraIntrinsics {
	return &PinholeCameraIntrinsics{
		Width:      0,
		Height:     0,
		Fx:         0,
		Fy:         0,
		Ppx:        0,
		Ppy:        0,
		Distortion: DistortionModel{},
	}
}
func NewDepthColorIntrinsicsExtrinsics() *DepthColorIntrinsicsExtrinsics {
	return &DepthColorIntrinsicsExtrinsics{
		ColorCamera:  PinholeCameraIntrinsics{0, 0, 0, 0, 0, 0, DistortionModel{0, 0, 0, 0, 0}},
		DepthCamera:  PinholeCameraIntrinsics{0, 0, 0, 0, 0, 0, DistortionModel{0, 0, 0, 0, 0}},
		ExtrinsicD2C: Extrinsics{[]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}, []float64{0, 0, 0}},
	}
}

func SetField(obj interface{}, name string, value interface{}) error {
	structValue := reflect.ValueOf(obj).Elem()
	structFieldValue := structValue.FieldByName(name)

	if !structFieldValue.IsValid() {
		return fmt.Errorf("No such field: %s in obj.", name)
	}

	if !structFieldValue.CanSet() {
		return fmt.Errorf("Cannot set %s field value.", name)
	}

	structFieldType := structFieldValue.Type()
	val := reflect.ValueOf(value)
	if structFieldType != val.Type() {
		return errors.New("Provided value type didn't match obj field type.")
	}

	structFieldValue.Set(val)
	return nil
}

func (s *DepthColorIntrinsicsExtrinsics) FillDepthColorIntrinsicsExtrinsicsFromMap(m map[string]interface{}) error {
	for k, v := range m {
		err := SetField(s, k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *PinholeCameraIntrinsics) FillPinholeCameraIntrinsicsFromMap(m map[string]interface{}) error {
	err := SetField(s, "Fx", m["fx"])
	if err != nil {
		return err
	}
	err = SetField(s, "Fy", m["fy"])
	if err != nil {
		return err
	}
	err = SetField(s, "ppx", m["ppx"])
	if err != nil {
		return err
	}
	err = SetField(s, "ppy", m["ppy"])
	if err != nil {
		return err
	}
	err = SetField(s, "Height", m["height"])
	if err != nil {
		return err
	}
	err = SetField(s, "Width", m["width"])
	if err != nil {
		return err
	}
	return nil
}

func NewDepthColorIntrinsicsExtrinsicsFromJsonFile(jsonPath string) *DepthColorIntrinsicsExtrinsics {
	intrinsics := NewDepthColorIntrinsicsExtrinsics()
	// open json file
	jsonFile, _ := os.Open(jsonPath)
	defer jsonFile.Close()
	// read our opened jsonFile as a byte array.
	byteValue, _ := ioutil.ReadAll(jsonFile)
	// Parse into map
	err := json.Unmarshal([]byte(byteValue), intrinsics)
	if err != nil {
		fmt.Printf("Error parsing JSON string - %s", err)
	}
	return intrinsics
}

func NewPinholeCameraIntrinsicsFromJsonFile(jsonPath, cameraName string) *PinholeCameraIntrinsics {
	intrinsics := NewDepthColorIntrinsicsExtrinsics()
	// open json file
	jsonFile, _ := os.Open(jsonPath)
	defer jsonFile.Close()
	// read our opened jsonFile as a byte array.
	byteValue, _ := ioutil.ReadAll(jsonFile)
	// Parse into map
	err := json.Unmarshal([]byte(byteValue), intrinsics)
	if err != nil {
		fmt.Printf("Error parsing JSON string - %s", err)
	}
	if cameraName == "depth" {
		return &intrinsics.DepthCamera
	}
	return &intrinsics.ColorCamera
}

// Function to transform a pixel with depth to a 3D point cloud
// the intrinsics parameters should be the ones of the sensor used to obtain the image that contains the pixel
func (params *PinholeCameraIntrinsics) PixelToPoint(x, y int, z float64) (float64, float64, float64) {
	//TODO(louise): add unit test
	xOverZ := (params.Ppx - float64(x)) / params.Fx
	yOverZ := (params.Ppy - float64(y)) / params.Fy
	// get x and y
	x_ := xOverZ * z
	y_ := yOverZ * z
	return x_, y_, z
}

// Function to project a 3D point to a pixel in an image plane
// the intrinsics parameters should be the ones of the sensor we want to project to
func (params *PinholeCameraIntrinsics) PointToPixel(x, y, z float64) (float64, float64) {
	//TODO(louise): add unit test
	if z != 0. {
		x_px := math.Round(x*params.Fx/(z) + params.Ppx)
		y_px := math.Round(y*params.Fy/(z) + params.Ppy)
		return x_px, y_px
	}
	// if depth is zero at this pixel, return negative coordinates so that the cropping to RGB bounds will filter it out
	return -1.0, -1.0
}

// Function to apply a rigid body transform between two cameras to a 3D point
func (params *Extrinsics) TransformPointToPoint(x, y, z float64) r3.Vector {
	//rotationMatrix translationVector
	rotationMatrix := params.RotationMatrix
	translationVector := params.TranslationVector
	n := len(rotationMatrix)
	if n != 9 {
		panic("Rotation Matrix to transform point cloud should be a 3x3 matrix")
	}
	x_transformed := rotationMatrix[0]*x + rotationMatrix[1]*y + rotationMatrix[2]*z + translationVector[0]
	y_transformed := rotationMatrix[3]*x + rotationMatrix[4]*y + rotationMatrix[5]*z + translationVector[1]
	z_transformed := rotationMatrix[6]*x + rotationMatrix[7]*y + rotationMatrix[8]*z + translationVector[2]

	return r3.Vector{x_transformed, y_transformed, z_transformed}
}
