package vision

import (
	"context"
	"fmt"
	"image"
	"path/filepath"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream/media"
	"go.uber.org/multierr"
)

type WebcamSource struct {
	reader media.VideoReadCloser
	depth  *webcamDepthSource
}

func NewWebcamSource(attrs map[string]string) (ImageDepthSource, error) {
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

	for i := 0; i <= 20; i++ {
		path := fmt.Sprintf("/dev/video%d", i)
		if debug {
			golog.Global.Debugf("%s", path)
		}

		s, err := tryWebcamOpen(path, debug, desiredSize)
		if err == nil {
			if debug {
				golog.Global.Debugf("\t USING")
			}

			if isIntel515 {
				s.depth, err = findWebcamDepth(debug)
				if err != nil {
					return nil, fmt.Errorf("found intel camera point no matching depth")
				}
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
	return media.VideoReadReleaser{s.reader}.Read()
}

func (s *WebcamSource) NextImageDepthPair(ctx context.Context) (image.Image, *DepthMap, error) {
	i, err := s.Next(ctx)
	var dm *DepthMap
	if s.depth != nil {
		dm, err = s.depth.Next()
		if err != nil {
			return nil, nil, err
		}
	}
	return i, dm, err
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

	return &WebcamSource{reader, nil}, nil
}
