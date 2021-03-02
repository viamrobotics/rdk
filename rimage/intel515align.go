package rimage

import (
	"fmt"
	"image"
	"time"
)

type Intel515Align struct {
}

func (i *Intel515Align) Align(ii *ImageWithDepth) (*ImageWithDepth, error) {

	if false {
		err := ii.WriteTo(fmt.Sprintf("data/align-test-%d.both.gz", time.Now().Unix()))
		if err != nil {
			return nil, err
		}
	}

	if ii.Color.Width() != 1280 || ii.Color.Height() != 720 ||
		ii.Depth.Width() != 1024 || ii.Depth.Height() != 768 {
		return nil, fmt.Errorf("unexpected intel dimensions c:(%d,%d) d:(%d,%d)",
			ii.Color.Width(), ii.Color.Height(), ii.Depth.Width(), ii.Depth.Height())
	}

	minX := 100
	minY := 120

	maxX := 950
	maxY := 680

	m := GetPerspectiveTransform(
		[]image.Point{
			{minX, minY},
			{maxX, minY},
			{minX, maxY},
			{maxX, maxY},
		},
		[]image.Point{
			{0, 0},
			{ii.Color.Width() - 1, 0},
			{0, ii.Color.Height() - 1},
			{ii.Color.Width() - 1, ii.Color.Height() - 1},
		},
	)

	dm2 := ii.Depth.Warp(m, ii.Color.Bounds().Max)
	return &ImageWithDepth{ii.Color, &dm2}, nil
}
