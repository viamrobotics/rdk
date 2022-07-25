package slam

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/rimage/transform"
)

const (
	// defines if images are RGB or BGR.
	rgbFlag = 1
	// stereo baseline in meters.
	stereoB = 0
	// stereo baselines used to classify a point as close or far.
	stereoThDepth = 30.0
	// factor to transform depth map to meters.
	depthMapFactor = 1000.0
	// file version needed by ORBSLAM.
	fileVersion = "1.0"
)

// orbCamMaker takes in the camera intrinsics and config params for orbslam and constructs a ORBsettings struct to use with yaml.Marshal.
func (slamSvc *slamService) orbCamMaker(intrinsics *transform.PinholeCameraIntrinsics) (*ORBsettings, error) {
	var err error

	orbslam := &ORBsettings{
		CamType:        "PinHole",
		Width:          intrinsics.Width,
		Height:         intrinsics.Height,
		Fx:             intrinsics.Fx,
		Fy:             intrinsics.Fy,
		Ppx:            intrinsics.Ppx,
		Ppy:            intrinsics.Ppy,
		RadialK1:       intrinsics.Distortion.RadialK1,
		RadialK2:       intrinsics.Distortion.RadialK2,
		RadialK3:       intrinsics.Distortion.RadialK3,
		TangentialP1:   intrinsics.Distortion.TangentialP1,
		TangentialP2:   intrinsics.Distortion.TangentialP2,
		RGBflag:        rgbFlag,
		Stereob:        stereoB,
		StereoThDepth:  stereoThDepth,
		DepthMapFactor: depthMapFactor,
		FPSCamera:      int8(slamSvc.dataRateMs),
		FileVersion:    fileVersion,
	}
	if orbslam.NFeatures, err = slamSvc.orbConfigToInt("orb_n_features", 1250); err != nil {
		return nil, err
	}
	if orbslam.ScaleFactor, err = slamSvc.orbConfigToFloat("orb_scale_factor", 1.2); err != nil {
		return nil, err
	}
	if orbslam.NLevels, err = slamSvc.orbConfigToInt("orb_n_levels", 8); err != nil {
		return nil, err
	}
	if orbslam.IniThFAST, err = slamSvc.orbConfigToInt("orb_n_ini_th_fast", 20); err != nil {
		return nil, err
	}
	if orbslam.MinThFAST, err = slamSvc.orbConfigToInt("orb_n_min_th_fast", 7); err != nil {
		return nil, err
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
	SaveMapLoc     string  `yaml:"System.SaveAtlasToFile"`
	LoadMapLoc     string  `yaml:"System.LoadAtlasFromFile"`
}

// generate a .yaml file to be used with orbslam.
func (slamSvc *slamService) orbGenYAML(ctx context.Context, cam camera.Camera) error {
	// Get the camera and check if the properties are valid
	proj, err := cam.GetProperties(ctx)
	if err != nil {
		return err
	}
	intrinsics, ok := proj.(*transform.PinholeCameraIntrinsics)
	if !ok {
		return transform.NewNoIntrinsicsError("Intrinsics do not exist")
	}
	err = intrinsics.CheckValid()
	if err != nil {
		return err
	}
	if err = intrinsics.CheckValid(); err != nil {
		return err
	}
	// create orbslam struct to generate yaml file with
	orbslam, err := slamSvc.orbCamMaker(intrinsics)
	if err != nil {
		return err
	}

	// TODO change time format to .Format(time.RFC3339Nano) https://viam.atlassian.net/browse/DATA-277
	// Check for maps in the specified directory and add map specifications to yaml config
	timeStamp, err := slamSvc.checkMaps()
	if err != nil {
		return errors.Wrap(err, "Error while checking for maps in dir")
	}
	if timeStamp == "" {
		timeStamp = time.Now().UTC().Format("2006-01-02T15_04_05.0000")
	} else {
		loadMapName := filepath.Join(slamSvc.dataDirectory, "map", slamSvc.cameraName+"_data_"+timeStamp)
		orbslam.LoadMapLoc = loadMapName
	}
	saveMapTimeStamp := time.Now().UTC().Format("2006-01-02T15_04_05.0000") // timestamp to save at end of run
	fileName := filepath.Join(slamSvc.dataDirectory, "config", slamSvc.cameraName+"_data_"+timeStamp+".yaml")
	saveMapName := filepath.Join(slamSvc.dataDirectory, "map", slamSvc.cameraName+"_data_"+saveMapTimeStamp)
	orbslam.SaveMapLoc = saveMapName

	// generate yaml file
	yamlData, err := yaml.Marshal(&orbslam)
	if err != nil {
		return errors.Wrap(err, "Error while Marshaling YAML file")
	}
	addLine := "%YAML:1.0\n"
	//nolint:gosec
	outfile, err := os.Create(fileName)
	if err != nil {
		return err
	}

	if _, err = outfile.WriteString(addLine); err != nil {
		return err
	}

	if _, err = outfile.Write(yamlData); err != nil {
		return err
	}
	return outfile.Close()
}

func (slamSvc *slamService) orbConfigToInt(key string, def int) (int, error) {
	valStr, ok := slamSvc.configParams[key]
	if !ok {
		slamSvc.logger.Debugf("Parameter %s not found, using default value %f", key, def)
		return def, nil
	}

	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, errors.Errorf("Parameter %s has an invalid definition", key)
	}

	return val, nil
}

func (slamSvc *slamService) orbConfigToFloat(key string, def float64) (float64, error) {
	valStr, ok := slamSvc.configParams[key]
	if !ok {
		slamSvc.logger.Debugf("Parameter %s not found, using default value %f", key, def)
		return def, nil
	}

	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0, errors.Errorf("Parameter %s has an invalid definition", key)
	}
	return val, nil
}

func (slamSvc *slamService) checkMaps() (string, error) {
	// var err error
	root := filepath.Join(slamSvc.dataDirectory, "map")
	mapExt := ".osa"
	mapTimestamp := time.Time{}
	var mapPath string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && filepath.Ext(path) == mapExt {
			// check if the file uses our format and grab timestamp if it does
			timestampLoc := strings.Index(path, "_data_") + len("_data_")
			if timestampLoc != -1 {
				timeFormat := "2006-01-02T15_04_05.0000"
				timestamp, err := time.Parse(timeFormat, path[timestampLoc:strings.Index(path, mapExt)])
				if err != nil {
					return err
				}
				if timestamp.After(mapTimestamp) {
					mapTimestamp = timestamp
					mapPath = path
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if mapPath == "" {
		slamSvc.logger.Debugf("No maps found in directory %s", root)
		return "", nil
	}

	return mapTimestamp.UTC().Format("2006-01-02T15_04_05.0000"), nil
}
