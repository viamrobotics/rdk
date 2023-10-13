//go:build !no_media

package segmentation

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
)

// ColorObjectsConfig specifies the necessary parameters for the color detection and transformation to 3D objects.
type ColorObjectsConfig struct {
	HueTolerance     float64 `json:"hue_tolerance_pct"`
	SaturationCutoff float64 `json:"saturation_cutoff_pct,omitempty"`
	ValueCutoff      float64 `json:"value_cutoff_pct,omitempty"`
	Color            string  `json:"detect_color"` // form #RRGGBB
	MeanK            int     `json:"mean_k"`       // used for StatisticalFilter
	Sigma            float64 `json:"sigma"`        // used for StatisticalFilter
	MinSegmentSize   int     `json:"min_points_in_segment"`
	Label            string  `json:"label,omitempty"`
}

// CheckValid checks to see in the input values are valid.
func (csc *ColorObjectsConfig) CheckValid() error {
	if csc.HueTolerance < 0.0 || csc.HueTolerance > 1.0 {
		return errors.Errorf("tolerance must be between 0.0 and 1.0, got %v", csc.HueTolerance)
	}
	var r, g, b uint8
	n, err := fmt.Sscanf(csc.Color, "#%02x%02x%02x", &r, &g, &b)
	if n != 3 || err != nil {
		return errors.Wrapf(err, "couldn't parse hex (%s) n: %d", csc.Color, n)
	}
	if csc.Sigma <= 0 {
		return errors.Errorf("sigma, the std dev used for filtering, must be greater than 0, got %v", csc.Sigma)
	}
	if csc.MinSegmentSize < 0 {
		return errors.Errorf("min_points_in_segment cannot be less than 0, got %v", csc.MinSegmentSize)
	}
	return nil
}

// ConvertAttributes changes the AttributeMap input into a ColorObjectsConfig.
func (csc *ColorObjectsConfig) ConvertAttributes(am utils.AttributeMap) error {
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

// ColorObjects returns a Segmenter that turns the bounding boxes found by the ColorDetector into 3D objects.
func ColorObjects(params utils.AttributeMap) (Segmenter, error) {
	cfg := &ColorObjectsConfig{}
	err := cfg.ConvertAttributes(params)
	if err != nil {
		return nil, err
	}
	// get info from config to build color detector
	detCfg := &objectdetection.ColorDetectorConfig{
		SegmentSize:       cfg.MinSegmentSize,
		HueTolerance:      cfg.HueTolerance,
		SaturationCutoff:  cfg.SaturationCutoff,
		ValueCutoff:       cfg.ValueCutoff,
		DetectColorString: cfg.Color,
		Label:             cfg.Label,
	}
	detector, err := objectdetection.NewColorDetector(detCfg)
	if err != nil {
		return nil, err
	}
	// turn the detector into a segmentor
	segmenter, err := DetectionSegmenter(detector, cfg.MeanK, cfg.Sigma, 1.)
	if err != nil {
		return nil, err
	}
	return segmenter, nil
}
