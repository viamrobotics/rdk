package calibration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"go.viam.com/robotcore/rimage"
)

func NewDepthColorIntrinsicsExtrinsicsFromJSONFile(jsonPath string) (*rimage.DepthColorIntrinsicsExtrinsics, error) {
	intrinsics := rimage.NewDepthColorIntrinsicsExtrinsics()
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

func NewPinholeCameraIntrinsicsFromJSONFile(jsonPath, cameraName string) (*rimage.PinholeCameraIntrinsics, error) {
	intrinsics := rimage.NewDepthColorIntrinsicsExtrinsics()
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
