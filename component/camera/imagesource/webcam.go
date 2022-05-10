package imagesource

import (
	"context"
	"path/filepath"
	"regexp"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream/media"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"webcam",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*WebcamAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return NewWebcamSource(attrs, logger)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "webcam",
		func(attributes config.AttributeMap) (interface{}, error) {
			cameraAttrs, err := camera.CommonCameraAttributes(attributes)
			if err != nil {
				return nil, err
			}
			var conf WebcamAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*WebcamAttrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			result.AttrConfig = cameraAttrs
			return result, nil
		}, &WebcamAttrs{})
}

// WebcamAttrs is the attribute struct for webcams.
type WebcamAttrs struct {
	*camera.AttrConfig
	Format      string `json:"format"`
	Path        string `json:"path"`
	PathPattern string `json:"path_pattern"`
}

func makeConstraints(attrs *WebcamAttrs, debug bool, logger golog.Logger) mediadevices.MediaStreamConstraints {
	minWidth := 0
	maxWidth := 4096
	idealWidth := 800
	minHeight := 0
	maxHeight := 2160
	idealHeight := 600

	if attrs.Width > 0 {
		minWidth = 0
		maxWidth = attrs.Width
		idealWidth = attrs.Width
	}

	if attrs.Height > 0 {
		minHeight = 0
		maxHeight = attrs.Height
		idealHeight = attrs.Height
	}

	return mediadevices.MediaStreamConstraints{
		Video: func(constraint *mediadevices.MediaTrackConstraints) {
			constraint.Width = prop.IntRanged{minWidth, maxWidth, idealWidth}
			constraint.Height = prop.IntRanged{minHeight, maxHeight, idealHeight}
			constraint.FrameRate = prop.FloatRanged{0, 200, 60}

			if attrs.Format == "" {
				constraint.FrameFormat = prop.FrameFormatOneOf{
					frame.FormatI420,
					frame.FormatI444,
					frame.FormatYUY2,
					frame.FormatUYVY,
					frame.FormatRGBA,
					frame.FormatMJPEG,
					frame.FormatNV12,
					frame.FormatNV21,
				}
			} else {
				constraint.FrameFormat = prop.FrameFormatExact(attrs.Format)
			}

			if debug {
				logger.Debugf("constraints: %v", constraint)
			}
		},
	}
}

// NewWebcamSource returns a new source based on a webcam discovered from the given attributes.
func NewWebcamSource(attrs *WebcamAttrs, logger golog.Logger) (camera.Camera, error) {
	var err error

	debug := attrs.Debug

	constraints := makeConstraints(attrs, debug, logger)

	if attrs.Path != "" {
		return tryWebcamOpen(attrs, attrs.Path, constraints)
	}

	var pattern *regexp.Regexp
	if attrs.PathPattern != "" {
		pattern, err = regexp.Compile(attrs.PathPattern)
		if err != nil {
			return nil, err
		}
	}

	labels := media.QueryVideoDeviceLabels()
	for _, label := range labels {
		if debug {
			logger.Debugf("%s", label)
		}

		if pattern != nil && !pattern.MatchString(label) {
			if debug {
				logger.Debug("\t skipping because of pattern")
			}
			continue
		}

		s, err := tryWebcamOpen(attrs, label, constraints)
		if err == nil {
			if debug {
				logger.Debug("\t USING")
			}

			return s, nil
		}
		if debug {
			logger.Debugf("\t %w", err)
		}
	}

	return nil, errors.New("found no webcams")
}

func tryWebcamOpen(attrs *WebcamAttrs, path string, constraints mediadevices.MediaStreamConstraints) (camera.Camera, error) {
	reader, err := media.GetNamedVideoReader(filepath.Base(path), constraints)
	if err != nil {
		return nil, err
	}
	return camera.New(reader, attrs.AttrConfig, nil)
}
