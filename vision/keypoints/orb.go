package keypoints

import (
	"encoding/json"
	"errors"
	"image"
	"os"
	"path/filepath"

	"go.viam.com/rdk/vision/keypoints/descriptors"
	"go.viam.com/utils"
)

// ORBConfig contains the parameters / configs needed to compute ORB features.
type ORBConfig struct {
	Layers          int          `json:"n_layers"`
	DownscaleFactor int          `json:"downscale_factor"`
	FastConf        *FASTConfig  `json:"fast"`
	BRIEFConf       *BRIEFConfig `json:"brief"`
}

// LoadORBConfiguration loads a ORBConfig from a json file.
func LoadORBConfiguration(file string) (*ORBConfig, error) {
	var config ORBConfig
	filePath := filepath.Clean(file)
	configFile, err := os.Open(filePath)
	defer utils.UncheckedErrorFunc(configFile.Close)
	if err != nil {
		return nil, err
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		return nil, err
	}
	err = config.Validate(file)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// Validate ensures all parts of the ORBConfig are valid.
func (config *ORBConfig) Validate(path string) error {
	if config.Layers < 1 {
		return utils.NewConfigValidationError(path, errors.New("n_layers should be >= 1"))
	}
	if config.DownscaleFactor <= 1 {
		return utils.NewConfigValidationError(path, errors.New("downscale_factor should be greater than 1"))
	}
	if config.FastConf == nil {
		return utils.NewConfigValidationFieldRequiredError(path, "fast")
	}
	if config.BRIEFConf == nil {
		return utils.NewConfigValidationFieldRequiredError(path, "brief")
	}
	return nil
}

// ComputeORBKeypoints compute ORB keypoints on gray image.
func ComputeORBKeypoints(im *image.Gray, cfg *ORBConfig) (descriptors.Descriptors, KeyPoints, error) {
	pyramid, err := GetImagePyramid(im)
	if err != nil {
		return nil, nil, err
	}
	if cfg.Layers <= 0 {
		err = errors.New("number of layers should be > 0")
		return nil, nil, err
	}
	if cfg.DownscaleFactor <= 1 {
		err = errors.New("number of layers should be >= 2")
		return nil, nil, err
	}
	if len(pyramid.Scales) < cfg.Layers {
		err = errors.New("more layers than actual number of octaves in image pyramid")
		return nil, nil, err
	}
	orbKeyPoints := make(descriptors.Descriptors, 0)
	orbPoints := make(KeyPoints, 0)
	for i := 0; i < cfg.Layers; i++ {
		currentImage := pyramid.Images[i]
		currentScale := pyramid.Scales[i]
		fastKps := NewFASTKeypointsFromImage(currentImage, cfg.FastConf)
		rescaledKps := RescaleKeypoints(fastKps.Points, currentScale)
		rescaledFASTKps := FASTKeypoints{
			Points:       rescaledKps,
			Orientations: fastKps.Orientations,
		}
		orbPoints = append(orbPoints, rescaledFASTKps.Points...)
		descs, err := ComputeBRIEFDescriptors(currentImage, &rescaledFASTKps, cfg.BRIEFConf)
		if err != nil {
			return nil, nil, err
		}
		orbKeyPoints = append(orbKeyPoints, descs...)
	}
	return orbKeyPoints, orbPoints, nil
}
