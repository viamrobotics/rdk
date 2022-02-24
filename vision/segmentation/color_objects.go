package segmentation

import (
	"context"

	"github.com/mitchellh/mapstructure"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/objectdetection"
)

type ColorObjectsConfig struct {
	Tolerance      float64 `json:"tolerance"`
	Color          string  `json:"detect_color"` // form #RRGGBB
	MeanK          int     `json:"mean_k"`
	Sigma          float64 `json:"sigma"`
	MinSegmentSize int     `json:"min_points_in_segment"`
}

func (csc *ColorObjectsConfig) CheckValid() error {
	return nil
}

func (csc *ColorObjectsConfig) ConvertAttributes(am config.AttributeMap) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: csc})
	if err != nil {
		return err
	}
	err = decoder.Decode(am)
	if err == nil {
		err = csc.CheckValid()
	}
	return err
}

// ColorObjects turns the bounding boxes found by the ColorDetector into 3D objects.
func ColorObjects(ctx context.Context, cam camera.Camera, params config.AttributeMap) ([]*vision.Object, error) {
	cfg := &ColorObjectsConfig{}
	err := cfg.ConvertAttributes(params)
	if err != nil {
		return nil, err
	}
	// get color from config to build color detector
	col, err := rimage.NewColorFromHex(cfg.Color)
	if err != nil {
		return nil, err
	}
	hue, _, _ := col.HsvNormal()
	det, err := objectdetection.NewColorDetector(cfg.Tolerance, hue)
	if err != nil {
		return nil, err
	}
	filter := objectdetection.NewAreaFilter(cfg.MinSegmentSize)
	detector, err := objectdetection.Build(nil, det, filter)
	if err != nil {
		return nil, err
	}
	proj := camera.Projector(cam)
	// turn the detector into a segmentor
	segmenter, err := DetectionSegmenter(detector, proj, cfg.MeanK, cfg.Sigma)
	if err != nil {
		return nil, err
	}
	return segmenter(ctx, cam, nil)

}
