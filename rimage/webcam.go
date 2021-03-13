package rimage

import (
	"context"
	"fmt"
	"image"
	"path/filepath"
	"regexp"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/media"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"

	"go.viam.com/robotcore/api"
)

func init() {
	api.RegisterCamera("webcam", func(r api.Robot, config api.Component) (gostream.ImageSource, error) {
		return NewWebcamSource(config.Attributes)
	})

}

func makeConstraints(attrs api.AttributeMap) mediadevices.MediaStreamConstraints {

	minWidth := 680
	maxWidth := 4096
	idealWidth := 1920
	minHeight := 400
	maxHeight := 2160
	idealHeight := 1080

	if attrs.Has("width") {
		w := attrs.GetInt("width", idealWidth)
		minWidth = w
		maxWidth = w
		idealWidth = w
	}

	if attrs.Has("height") {
		h := attrs.GetInt("height", idealHeight)
		minHeight = h
		maxHeight = h
		idealHeight = h
	}

	return mediadevices.MediaStreamConstraints{
		Video: func(constraint *mediadevices.MediaTrackConstraints) {

			constraint.Width = prop.IntRanged{minWidth, maxWidth, idealWidth}
			constraint.Height = prop.IntRanged{minHeight, maxHeight, idealHeight}
			constraint.FrameRate = prop.FloatRanged{0, 200, 60}

			if !attrs.Has("format") || attrs.GetString("format") == "" {
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
				constraint.FrameFormat = prop.FrameFormatExact(attrs.GetString("format"))
			}
		},
	}
}

type Aligner interface {
	Align(ctx context.Context, img *ImageWithDepth) (*ImageWithDepth, error)
}

func NewWebcamSource(attrs api.AttributeMap) (gostream.ImageSource, error) {
	var err error

	debug := attrs.GetBool("debug", false)

	constraints := makeConstraints(attrs)

	if debug {
		golog.Global.Debugf("constraints: %v", constraints)
	}

	if attrs.Has("path") {
		return tryWebcamOpen(attrs.GetString("path"), debug, constraints)
	}

	var pattern *regexp.Regexp
	if attrs.Has("path_pattern") {
		pattern, err = regexp.Compile(attrs.GetString("path_pattern"))
		if err != nil {
			return nil, err
		}
	}

	labels := media.QueryVideoDeviceLabels()
	for _, label := range labels {
		if debug {
			golog.Global.Debugf("%s", label)
		}

		if pattern != nil && !pattern.MatchString(label) {
			if debug {
				golog.Global.Debugf("\t skipping because of pattern")
			}
			continue
		}

		s, err := tryWebcamOpen(label, debug, constraints)
		if err == nil {
			if debug {
				golog.Global.Debugf("\t USING")
			}

			return s, nil
		}
		if debug {
			golog.Global.Debugf("\t %s", err)
		}
	}

	return nil, fmt.Errorf("could find no webcams")
}

func tryWebcamOpen(path string, debug bool, constraints mediadevices.MediaStreamConstraints) (gostream.ImageSource, error) {
	return media.GetNamedVideoReader(filepath.Base(path), constraints)
}

func imageToDepthMap(img image.Image) *DepthMap {
	bounds := img.Bounds()

	width, height := bounds.Dx(), bounds.Dy()

	// TODO(erd): handle non realsense Z16 devices better
	// realsense seems to rotate
	dm := NewEmptyDepthMap(height, width)

	grayImg := img.(*image.Gray16)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			i := grayImg.PixOffset(x, y)
			z := uint16(grayImg.Pix[i+0])<<8 | uint16(grayImg.Pix[i+1])
			dm.Set(y, x, Depth(z))
		}
	}

	return &dm
}
