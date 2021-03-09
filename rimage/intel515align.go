package rimage

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/edaniels/golog"

	"go.opencensus.io/trace"

	"github.com/edaniels/gostream"
)

type depthComposed struct {
	color gostream.ImageSource
	depth gostream.ImageSource
}

func NewDepthComposed(color, depth gostream.ImageSource) (gostream.ImageSource, error) {
	if color == nil {
		return nil, fmt.Errorf("need color")
	}
	if depth == nil {
		return nil, fmt.Errorf("need depth")
	}
	return &depthComposed{color, depth}, nil
}

func (dc *depthComposed) Close() error {
	// TODO(erh): who owns these?
	return nil
}

func (dc *depthComposed) Next(ctx context.Context) (image.Image, func(), error) {
	c, cCloser, err := dc.color.Next(ctx)
	if err != nil {
		return nil, nil, err
	}

	defer cCloser()

	d, dCloser, err := dc.depth.Next(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer dCloser()

	aligned, err := intel515align(&ImageWithDepth{ConvertImage(c), imageToDepthMap(d)})

	return aligned, func() {}, err

}

type Intel515Align struct {
	currentlyWriting bool
}

func (i *Intel515Align) Align(ctx context.Context, ii *ImageWithDepth) (*ImageWithDepth, error) {
	_, span := trace.StartSpan(ctx, "Intel515Align")
	defer span.End()

	if false {
		if !i.currentlyWriting {
			i.currentlyWriting = true
			go func() {
				defer func() { i.currentlyWriting = false }()
				fn := fmt.Sprintf("data/align-test-%d.both.gz", time.Now().Unix())
				err := ii.WriteTo(fn)
				if err != nil {
					golog.Global.Debugf("error writing debug file: %s", err)
				} else {
					golog.Global.Debugf("wrote debug file to %s", fn)
				}
			}()
		}
		return ii, nil
	}

	return intel515align(ii)
}

func rectToPoints(r image.Rectangle) []image.Point {
	return []image.Point{
		r.Min,
		{r.Max.X, r.Min.Y},
		{r.Min.X, r.Max.Y},
		r.Max,
	}
}

func intel515align(ii *ImageWithDepth) (*ImageWithDepth, error) {
	if ii.Color.Width() != 1280 || ii.Color.Height() != 720 ||
		ii.Depth.Width() != 1024 || ii.Depth.Height() != 768 {
		return nil, fmt.Errorf("unexpected intel dimensions c:(%d,%d) d:(%d,%d)",
			ii.Color.Width(), ii.Color.Height(), ii.Depth.Width(), ii.Depth.Height())
	}

	newWidth := 640
	newHeight := 360
	newBounds := image.Rect(0, 0, newWidth, newHeight)

	dst := rectToPoints(newBounds)

	depthPoints := rectToPoints(image.Rect(67, 100, 1019, 665))
	colorPoints := rectToPoints(image.Rect(0, 0, 1196, ii.Color.Height()))

	c2 := WarpImage(ii, GetPerspectiveTransform(colorPoints, dst), newBounds.Max)
	dm2 := ii.Depth.Warp(GetPerspectiveTransform(depthPoints, dst), newBounds.Max)

	return &ImageWithDepth{c2, &dm2}, nil
}
