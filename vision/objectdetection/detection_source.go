package objectdetection

import (
	"context"
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.viam.com/utils"

	"go.viam.com/rdk/rimage"
)

// Result holds all useful information for the detector: contains the original image, the preprocessed image, and the final detections.
type Result struct {
	OriginalImage image.Image
	Detections    []Detection
	Release       func()
	Err           error
}

// Source pulls an image from src and applies the detector pipeline to it, resulting in an image overlaid with detections.
// Fulfills gostream.ImageSource interface.
type Source struct {
	pipelineOutput          chan *Result
	activeBackgroundWorkers sync.WaitGroup
	cancelCtx               context.Context
	cancelFunc              func()
}

// NewSource builds the pipeline from an input ImageSource and Detector.
func NewSource(src gostream.ImageSource, det Detector) (*Source, error) {
	// fill optional functions with identity operators
	if src == nil {
		return nil, errors.New("object detection source must include an image source to pull from")
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	if det == nil {
		det = func(ctx context.Context, img image.Image) ([]Detection, error) { return nil, nil }
	}

	s := &Source{
		pipelineOutput: make(chan *Result),
		cancelCtx:      cancelCtx,
		cancelFunc:     cancel,
	}

	s.backgroundWorker(src, det)
	return s, nil
}

func (s *Source) backgroundWorker(src gostream.ImageSource, det Detector) {
	// define the full pipeline
	s.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for {
			original, release, err := src.Next(s.cancelCtx)
			if err != nil && errors.Is(err, context.Canceled) {
				return
			}
			clone := rimage.CloneImage(original)
			detections, err := det(s.cancelCtx, clone)

			r := &Result{
				OriginalImage: clone,
				Detections:    detections,
				Release:       release,
				Err:           err,
			}
			select {
			case <-s.cancelCtx.Done():
				return
			case s.pipelineOutput <- r:
				// do nothing
			}
		}
	}, s.activeBackgroundWorkers.Done)
}

// Close closes all the channels and threads.
func (s *Source) Close() {
	s.cancelFunc()
	s.activeBackgroundWorkers.Wait()
}

// Next returns the original image overlaid with the found detections.
func (s *Source) Next(ctx context.Context) (image.Image, func(), error) {
	ctx, span := trace.StartSpan(ctx, "vision::objectdetection::Source::Next")
	defer span.End()
	start := time.Now()
	res, err := s.NextResult(ctx)
	if err != nil {
		return nil, nil, err
	}
	duration := time.Since(start)
	fps := 1. / duration.Seconds()
	ovImg, err := Overlay(res.OriginalImage, res.Detections)
	if err != nil {
		return nil, nil, err
	}
	ovImg = OverlayText(ovImg, fmt.Sprintf("FPS: %.2f", fps))
	return ovImg, res.Release, nil
}

// NextResult returns all the components required to build the overlaid image, but is useful if you only want the Detections.
func (s *Source) NextResult(ctx context.Context) (*Result, error) {
	ctx, span := trace.StartSpan(ctx, "vision::objectdetection::Source::NextResult")
	defer span.End()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.cancelCtx.Done():
		return nil, s.cancelCtx.Err()
	case result := <-s.pipelineOutput:
		return result, result.Err
	}
}
