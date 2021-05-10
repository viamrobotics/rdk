package imagesource

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/rimage"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/media"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
)

func init() {
	api.RegisterCamera("webcam", func(ctx context.Context, r api.Robot, config api.ComponentConfig, logger golog.Logger) (gostream.ImageSource, error) {
		return NewWebcamSource(config.Attributes, logger)
	})

}

func makeConstraints(attrs api.AttributeMap, debug bool, logger golog.Logger) mediadevices.MediaStreamConstraints {

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

type Aligner interface {
	Align(ctx context.Context, img *rimage.ImageWithDepth) (*rimage.ImageWithDepth, error)
}

func NewWebcamSource(attrs api.AttributeMap, logger golog.Logger) (gostream.ImageSource, error) {
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
				logger.Debugf("\t skipping because of pattern")
			}
			continue
		}

		s, err := tryWebcamOpen(label, debug, constraints)
		if err == nil {
			if debug {
				logger.Debugf("\t USING")
			}

			return s, nil
		}
		if debug {
			logger.Debugf("\t %s", err)
		}
	}

	return nil, fmt.Errorf("found no webcams")
}

func tryWebcamOpen(path string, debug bool, constraints mediadevices.MediaStreamConstraints) (gostream.ImageSource, error) {
	return media.GetNamedVideoReader(filepath.Base(path), constraints)
}
