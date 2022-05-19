package keypoints

import (
	"encoding/json"
	"errors"
	"go.viam.com/utils"
	"image"
	"os"
	"path/filepath"
)

type ORBConfig struct {
	Layers          int          `json:"n_layers"`
	DownscaleFactor int          `json:"downscale_factor"`
	FastConf        *FASTConfig  `json:"fast"`
	BRIEFConf       *BRIEFConfig `json:"brief"`
}

// LoadORBConfiguration loads a ORBConfig from a json file.
func LoadORBConfiguration(file string) *ORBConfig {
	var config ORBConfig
	filePath := filepath.Clean(file)
	configFile, err := os.Open(filePath)
	defer utils.UncheckedErrorFunc(configFile.Close)
	if err != nil {
		return nil
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		return nil
	}
	return &config
}

// ComputeORBKeypoints compute ORB keypoints on gray image
func ComputeORBKeypoints(im *image.Gray, cfg *ORBConfig) (Descriptors, error) {
	pyramid, err := GetImagePyramid(im)
	if err != nil {
		return nil, err
	}
	if cfg.Layers <= 0 {
		err = errors.New("number of layers should be > 0")
		return nil, err
	}
	if cfg.DownscaleFactor <= 1 {
		err = errors.New("number of layers should be >= 2")
		return nil, err
	}
	if len(pyramid.Scales) < cfg.Layers {
		err = errors.New("more layers than actual number of octaves in image pyramid")
		return nil, err
	}
	orbKeyPoints := make(Descriptors, 0)
	for i := 0; i < cfg.Layers; i++ {
		currentImage := pyramid.Images[i]
		currentScale := pyramid.Scales[i]
		fastKps := NewFASTKeypointsFromImage(currentImage, cfg.FastConf)
		rescaledKps := RescaleKeypoints(fastKps.Points, currentScale)
		rescaledFASTKps := FASTKeypoints{
			Points:       rescaledKps,
			Orientations: fastKps.Orientations,
		}
		descs, err := ComputeBRIEFDescriptors(currentImage, &rescaledFASTKps, cfg.BRIEFConf)
		if err != nil {
			return nil, err
		}
		orbKeyPoints = append(orbKeyPoints, descs...)
	}
	return orbKeyPoints, nil
}
