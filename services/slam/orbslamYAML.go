// Package slam implements simultaneous localization and mapping
package slam

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/rimage/transform"
)

// orbCamMaker takes in the camera intrinsics and config params for orbslam and constructs a ORBsettings struct to use with yaml.Marshal.
func orbCamMaker(intrinics *transform.PinholeCameraIntrinsics, slamSvc *slamService) (orbslam ORBsettings, err error) {
	orbslam = ORBsettings{
		CamType:        "PinHole",
		Width:          intrinics.Width,
		Height:         intrinics.Height,
		Fx:             intrinics.Fx,
		Fy:             intrinics.Fy,
		Ppx:            intrinics.Ppx,
		Ppy:            intrinics.Ppy,
		RadialK1:       intrinics.Distortion.RadialK1,
		RadialK2:       intrinics.Distortion.RadialK2,
		RadialK3:       intrinics.Distortion.RadialK3,
		TangentialP1:   intrinics.Distortion.TangentialP1,
		TangentialP2:   intrinics.Distortion.TangentialP2,
		RGBflag:        1,
		Stereob:        0,
		StereoThDepth:  40.0,
		DepthMapFactor: 1000.0,
		FPSCamera:      int8(slamSvc.dataRateMs),
		FileVersion:    "1.0",
	}
	orbslam.NFeatures, err = orbConfigToInt(slamSvc.configParams["orb_n_features"], 1250)
	if err != nil {
		return orbslam, err
	}
	orbslam.ScaleFactor, err = orbConfigToFloat(slamSvc.configParams["orb_scale_factor"], 1.2)
	if err != nil {
		return orbslam, err
	}
	orbslam.NLevels, err = orbConfigToInt(slamSvc.configParams["orb_n_levels"], 8)
	if err != nil {
		return orbslam, err
	}
	orbslam.IniThFAST, err = orbConfigToInt(slamSvc.configParams["orb_n_ini_th_fast"], 20)
	if err != nil {
		return orbslam, err
	}
	orbslam.MinThFAST, err = orbConfigToInt(slamSvc.configParams["orb_n_min_th_fast"], 7)
	if err != nil {
		return orbslam, err
	}
	if err != nil {
		return orbslam, err
	}
	return orbslam, nil
}

// ORBsettings is used to construct the yaml file.
type ORBsettings struct {
	FileVersion    string  `yaml:"File.version"`
	NFeatures      int     `yaml:"ORBextractor.nFeatures"`
	ScaleFactor    float64 `yaml:"ORBextractor.scaleFactor"`
	NLevels        int     `yaml:"ORBextractor.nLevels"`
	IniThFAST      int     `yaml:"ORBextractor.iniThFAST"`
	MinThFAST      int     `yaml:"ORBextractor.minThFAST"`
	CamType        string  `yaml:"Camera.type"`
	Width          int     `yaml:"Camera.width"`
	Height         int     `yaml:"Camera.height"`
	Fx             float64 `yaml:"Camera1.fx"`
	Fy             float64 `yaml:"Camera1.fy"`
	Ppx            float64 `yaml:"Camera1.cx"`
	Ppy            float64 `yaml:"Camera1.cy"`
	RadialK1       float64 `yaml:"Camera1.k1"`
	RadialK2       float64 `yaml:"Camera1.k2"`
	RadialK3       float64 `yaml:"Camera1.k3"`
	TangentialP1   float64 `yaml:"Camera1.p1"`
	TangentialP2   float64 `yaml:"Camera1.p2"`
	RGBflag        int8    `yaml:"Camera.RGB"`
	Stereob        float32 `yaml:"Stereo.b"`
	StereoThDepth  float32 `yaml:"Stereo.ThDepth"`
	DepthMapFactor float32 `yaml:"RGBD.DepthMapFactor"`
	FPSCamera      int8    `yaml:"Camera.fps"`
}

// generate a .yaml file to be used with orbslam.
func orbGenYAML(slamSvc *slamService, cam camera.Camera) error {
	proj := camera.Projector(cam) // will be nil if no intrinsics
	if proj == nil {
		return errors.New("error camera intrinsics were not defined properly")
	}
	intrinsics, ok := proj.(*transform.PinholeCameraIntrinsics)
	if !ok {
		return errors.New("error camera intrinsics were not defined properly")
	}
	orbslam, err := orbCamMaker(intrinsics, slamSvc)
	if err != nil {
		return err
	}
	orbslam.FileVersion = "1.0"
	yamlData, err := yaml.Marshal(&orbslam)
	if err != nil {
		return errors.Errorf("Error while Marshaling YAML file. %v", err)
	}

	timeStamp := time.Now().UTC().Format("2006-01-02T15_04_05.0000")
	fileName := filepath.Join(slamSvc.dataDirectory, "config", slamSvc.cameraName+"_data_"+timeStamp+".yaml")

	addLine := "%YAML:1.0\n"
	//nolint:gosec
	outfile, err := os.Create(fileName)
	if err != nil {
		return err
	}
	_, err = outfile.WriteString(addLine)
	if err != nil {
		return err
	}
	_, err = outfile.Write(yamlData)
	if err != nil {
		return err
	}
	return outfile.Close()
}

func orbConfigToInt(attr string, def int) (int, error) {
	var val int
	if attr == "" {
		val = def
	} else {
		var err error
		val, err = strconv.Atoi(attr)
		if err != nil {
			return val, err
		}
	}
	return val, nil
}

func orbConfigToFloat(attr string, def float64) (float64, error) {
	var val float64
	if attr == "" {
		val = def
	} else {
		var err error
		val, err = strconv.ParseFloat(attr, 64)
		if err != nil {
			return val, err
		}
	}
	return val, nil
}
