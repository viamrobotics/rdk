package imagesource

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"color_detector",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*colorDetectorAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			sourceName := attrs.Source
			src, err := camera.FromRobot(r, sourceName)
			if err != nil {
				return nil, fmt.Errorf("no source camera (%s): %w", sourceName, err)
			}
			return newColorDetector(src, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(
		camera.SubtypeName,
		"color_detector",
		func(attributes config.AttributeMap) (interface{}, error) {
			cameraAttrs, err := camera.CommonCameraAttributes(attributes)
			if err != nil {
				return nil, err
			}
			var conf colorDetectorAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*colorDetectorAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			result.AttrConfig = cameraAttrs
			return result, nil
		},
		&colorDetectorAttrs{},
	)
}

// colorDetectorAttrs is the attribute struct for color detectors.
type colorDetectorAttrs struct {
	*camera.AttrConfig
	SegmentSize       int      `json:"segment_size"`
	Tolerance         float64  `json:"tolerance"`
	ExcludeColors     []string `json:"exclude_color_chans"`
	DetectColorString string   `json:"detect_color"`
}

// DetectColor transforms the color hexstring into a slice of uint8.
func (ac *colorDetectorAttrs) DetectColor() ([]uint8, error) {
	if ac.DetectColorString == "" {
		return []uint8{}, nil
	}
	pound, color := ac.DetectColorString[0], ac.DetectColorString[1:]
	if pound != '#' {
		return nil, errors.Errorf("detect_color is ill-formed, expected #RRGGBB, got %v", ac.DetectColorString)
	}
	slice, err := hex.DecodeString(color)
	if err != nil {
		return nil, err
	}
	if len(slice) != 3 {
		return nil, errors.Errorf("detect_color is ill-formed, expected #RRGGBB, got %v", ac.DetectColorString)
	}
	return slice, nil
}

// newColorDetector creates a simple color detector from a source camera component in the config and user defined attributes.
func newColorDetector(src camera.Camera, attrs *colorDetectorAttrs) (camera.Camera, error) {
	// define the preprocessor
	pSlice := make([]objectdetection.Preprocessor, 0, 3)
	for _, c := range attrs.ExcludeColors {
		rc, err := objectdetection.RemoveColorChannel(c)
		if err != nil {
			return nil, err
		}
		pSlice = append(pSlice, rc)
	}
	p := objectdetection.ComposePreprocessors(pSlice)

	// define the detector
	tolerance := 0.05 // default value of 5%
	if attrs.Tolerance != 0. {
		tolerance = attrs.Tolerance
	}
	detCfg := &objectdetection.ColorDetectorConfig{
		SegmentSize:       attrs.SegmentSize,
		Tolerance:         tolerance,
		DetectColorString: attrs.DetectColorString,
	}
	d, err := objectdetection.NewColorDetector(detCfg)
	if err != nil {
		return nil, err
	}

	det, err := objectdetection.Build(p, d, nil)
	if err != nil {
		return nil, err
	}
	detector, err := objectdetection.NewSource(src, det)
	if err != nil {
		return nil, err
	}
	return camera.New(detector, attrs.AttrConfig, src)
}
