package imagesource

import (
	"context"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream/media"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
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
			return NewWebcamSource(config.Attributes, logger)
		}})

}

func makeConstraints(attrs config.AttributeMap, debug bool, logger golog.Logger) mediadevices.MediaStreamConstraints {

	minWidth := 680
	maxWidth := 4096
	idealWidth := 1920
	minHeight := 400
	maxHeight := 2160
	idealHeight := 1080

	if attrs.Has("width") {
		w := attrs.Int("width", idealWidth)
		minWidth = w
		maxWidth = w
		idealWidth = w
	}

	if attrs.Has("height") {
		h := attrs.Int("height", idealHeight)
		minHeight = h
		maxHeight = h
		idealHeight = h
	}

	return mediadevices.MediaStreamConstraints{
		Video: func(constraint *mediadevices.MediaTrackConstraints) {

			constraint.Width = prop.IntRanged{minWidth, maxWidth, idealWidth}
			constraint.Height = prop.IntRanged{minHeight, maxHeight, idealHeight}
			constraint.FrameRate = prop.FloatRanged{0, 200, 60}

			if !attrs.Has("format") || attrs.String("format") == "" {
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
				constraint.FrameFormat = prop.FrameFormatExact(attrs.String("format"))
			}

			if debug {
				logger.Debugf("constraints: %v", constraint)
			}
		},
	}
}

// NewWebcamSource returns a new source based on a webcam discovered from the given attributes.
func NewWebcamSource(attrs config.AttributeMap, logger golog.Logger) (camera.Camera, error) {
	var err error

	debug := attrs.Bool("debug", false)

	constraints := makeConstraints(attrs, debug, logger)

	if attrs.Has("path") {
		return tryWebcamOpen(attrs.String("path"), debug, constraints)
	}

	var pattern *regexp.Regexp
	if attrs.Has("path_pattern") {
		pattern, err = regexp.Compile(attrs.String("path_pattern"))
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

		s, err := tryWebcamOpen(label, debug, constraints)
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

func tryWebcamOpen(path string, debug bool, constraints mediadevices.MediaStreamConstraints) (camera.Camera, error) {
	reader, err := media.GetNamedVideoReader(filepath.Base(path), constraints)
	if err != nil {
		return nil, err
	}
	return &camera.ImageSource{ImageSource: reader}, nil
}
