package rimage

import (
	"context"
	"fmt"
	"image"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/media"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
)

func makeConstraints(attrs map[string]string) mediadevices.MediaStreamConstraints {

	minWidth := 680
	maxWidth := 4096
	idealWitdh := 1920

	if attrs["width"] != "" {
		w, err := strconv.Atoi(attrs["width"])
		if err != nil {
			golog.Global.Warnf("bad width %s", err)
		} else {
			minWidth = w
			maxWidth = w
			idealWitdh = w
		}
	}

	return mediadevices.MediaStreamConstraints{
		Video: func(constraint *mediadevices.MediaTrackConstraints) {

			constraint.Width = prop.IntRanged{minWidth, maxWidth, idealWitdh}
			constraint.Height = prop.IntRanged{400, 2160, 1080}
			constraint.FrameRate = prop.FloatRanged{0, 200, 60}

			format := attrs["format"]
			if format == "" {
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
				constraint.FrameFormat = prop.FrameFormatExact(format)
			}
		},
	}
}

type Aligner interface {
	Align(ctx context.Context, img *ImageWithDepth) (*ImageWithDepth, error)
}

func NewWebcamSource(attrs map[string]string) (gostream.ImageSource, error) {
	var err error

	debug := attrs["debug"] == "true"

	constraints := makeConstraints(attrs)

	path := attrs["path"]
	if path != "" {
		return tryWebcamOpen(path, debug, constraints)
	}

	var pattern *regexp.Regexp
	pathPattern := attrs["path_pattern"]
	if pathPattern != "" {
		pattern, err = regexp.Compile(pathPattern)
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
			dm.Set(height-y-1, width-x-1, Depth(z))
		}
	}

	return &dm
}
