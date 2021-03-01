package rimage

import (
	"image"
)

type Intel515Align struct {
}

func (i *Intel515Align) Align(ii *ImageWithDepth) (*ImageWithDepth, error) {
	minX := 0
	minY := 120

	maxX := 900
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
