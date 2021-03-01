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
	"go.uber.org/multierr"
)

type Aligner interface {
	Align(img *ImageWithDepth) (*ImageWithDepth, error)
}

type WebcamSource struct {
	reader media.VideoReadCloser
	depth  *webcamDepthSource
	align  Aligner
}

func NewWebcamSource(attrs map[string]string) (gostream.ImageSource, error) {
	debug := attrs["debug"] == "true"
	var desiredSize *image.Point = nil

	// TODO(erh): this is gross, but not sure what right config options are yet
	isIntel515 := attrs["model"] == "intel515"

	if isIntel515 {
		desiredSize = &image.Point{1280, 720}
	}

	path := attrs["path"]
	if path != "" {
		return tryWebcamOpen(path, debug, desiredSize)
	}

	pathPattern := attrs["path_pattern"]
	if pathPattern != "" {
		pattern, err := regexp.Compile(pathPattern)
		if err != nil {
			return nil, err
		}
		return tryWebcamOpenPattern(pattern, debug, desiredSize)
	}

	labels := media.QueryVideoDeviceLabels()
	for _, label := range labels {
		if debug {
			golog.Global.Debugf("%s", label)
		}

		s, err := tryWebcamOpen(label, debug, desiredSize)
		if err == nil {
			if debug {
				golog.Global.Debugf("\t USING")
			}

			if isIntel515 {
				s.depth, err = findWebcamDepth(debug)
				if err != nil {
					return nil, fmt.Errorf("found intel camera point no matching depth")
				}
				s.align = &Intel515Align{}
			}

			return s, nil
		}
		if debug {
			golog.Global.Debugf("\t %s", err)
		}
	}

	return nil, fmt.Errorf("could find no webcams")
}

func (s *WebcamSource) Next(ctx context.Context) (image.Image, error) {
	img, err := media.VideoReadReleaser{s.reader}.Read()
	if err != nil {
		return nil, err
	}

	if s.depth == nil {
		return img, nil
	}

	dm, err := s.depth.Next()
	if err != nil {
		return nil, err
	}
	return s.align.Align(&ImageWithDepth{ConvertImage(img), dm})
}

func (s *WebcamSource) Close() error {
	var errs []error
	if s.depth != nil {
		if err := s.depth.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := s.reader.Close(); err != nil {
		errs = append(errs, err)
	}
	return multierr.Combine(errs...)
}

func tryWebcamOpen(path string, debug bool, desiredSize *image.Point) (*WebcamSource, error) {
	reader, err := media.GetNamedVideoReader(filepath.Base(path), media.DefaultConstraints)
	if err != nil {
		return nil, err
	}

	return &WebcamSource{reader, nil, nil}, nil
}

func tryWebcamOpenPattern(pattern *regexp.Regexp, debug bool, desiredSize *image.Point) (*WebcamSource, error) {
	reader, err := media.GetPatternedVideoReader(pattern, media.DefaultConstraints)
	if err != nil {
		return nil, err
	}

	return &WebcamSource{reader, nil, nil}, nil
}
