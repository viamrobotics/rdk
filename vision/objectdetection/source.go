package objectdetection

import (
	"context"
	"image"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
)

// Source "pulls" an image from src and applies the detector pipeline to it, resulting in an image overlaid with detections.
// Fulfills gostream.ImageSource interface.
type Source struct {
	src  gostream.ImageSource
	prep Preprocessor
	det  Detector
	filt Filter
}

// Result all useful information for the detector: contains the original image, the preprocessed image, and the final detections.
type Result struct {
	OriginalImage     image.Image
	PreprocessedImage image.Image
	Detections        []Detection
}

// NewSource builds the pipeline from an input ImageSource, Preprocessor (optional), Detector (required) and  Filter (optional).
func NewSource(src gostream.ImageSource, prep Preprocessor, det Detector, filt Filter) (*Source, error) {
	if src == nil {
		return nil, errors.New("object detection source must include an image source to pull from")
	}
	if det == nil {
		return nil, errors.New("object detector function cannot be nil")
	}
	return &Source{src, prep, det, filt}, nil
}

// Next returns the original image overlaid with the found detections.
func (s *Source) Next(ctx context.Context) (image.Image, func(), error) {
	res, err := s.NextResult(ctx)
	if err != nil {
		return nil, nil, err
	}
	ovImg, err := Overlay(res.OriginalImage, res.Detections)
	if err != nil {
		return nil, nil, err
	}
	return ovImg, func() {}, nil
}

// NextResult returns all the components required to build the overlaid image, but is useful if you only want the Detections.
func (s *Source) NextResult(ctx context.Context) (*Result, error) {
	var err error
	r := &Result{}
	r.OriginalImage, _, err = s.src.Next(ctx)
	if err != nil {
		return nil, err
	}
	//r.PreprocessedImage = CopyImage(r.OriginalImage)
	r.PreprocessedImage = r.OriginalImage
	if s.prep != nil {
		r.PreprocessedImage = s.prep(r.PreprocessedImage)
	}
	r.Detections, err = s.det(r.PreprocessedImage)
	if err != nil {
		return nil, err
	}
	if s.filt != nil {
		r.Detections = s.filt(r.Detections)
	}
	return r, nil
}
