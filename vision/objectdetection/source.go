package objectdetection

import (
	"context"
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/vision"
)

// Result holds all useful information for the detector: contains the original image, the preprocessed image, and the final detections.
type Result struct {
	OriginalImage     image.Image
	PreprocessedImage image.Image
	Detections        []Detection
	Release           func()
	Err               error
}

// Source pulls an image from src and applies the detector pipeline to it, resulting in an image overlaid with detections.
// Fulfills gostream.ImageSource interface.
type Source struct {
	pipelineOutput          chan *Result
	activeBackgroundWorkers sync.WaitGroup
	proj                    rimage.Projector
	cancelCtx               context.Context
	cancelFunc              func()
}

// NewSource builds the pipeline from an input ImageSource, Preprocessor, Detector and  Postprocessor.
func NewSource(src gostream.ImageSource, proj rimage.Projector, prep Preprocessor, det Detector, post Postprocessor) (*Source, error) {
	// fill optional functions with identity operators
	if src == nil {
		return nil, errors.New("object detection source must include an image source to pull from")
	}
	if prep == nil {
		prep = func(img image.Image) image.Image { return img }
	}
	if det == nil {
		det = func(img image.Image) ([]Detection, error) { return nil, nil }
	}
	if post == nil {
		post = func(inp []Detection) []Detection { return inp }
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	s := &Source{
		pipelineOutput: make(chan *Result),
		proj:           proj,
		cancelCtx:      cancelCtx,
		cancelFunc:     cancel,
	}

	s.backgroundWorker(src, prep, det, post)
	return s, nil
}

func (s *Source) backgroundWorker(src gostream.ImageSource, prep Preprocessor, det Detector, post Postprocessor) {
	// define the full pipeline
	s.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for {
			original, release, err := src.Next(s.cancelCtx)
			if err != nil && errors.Is(err, context.Canceled) {
				return
			}
			clone := rimage.CloneToImageWithDepth(original) // use depth info if available
			preprocessed := prep(clone)
			detections, err := det(preprocessed)
			if err == nil {
				detections = post(detections)
			}

			r := &Result{
				OriginalImage:     clone,
				PreprocessedImage: preprocessed,
				Detections:        detections,
				Release:           release,
				Err:               err,
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

// NextObjects returns the 3D objects in the scene if there Projector for the camera.
func (s *Source) NextObjects(ctx context.Context, conf *vision.Parameters3D) ([]*vision.Object, error) {
	res, err := s.NextResult(ctx)
	if err != nil {
		return nil, err
	}
	return ToObjects(res.Detections, rimage.ConvertToImageWithDepth(res.OriginalImage), s.proj)
}

// NextResult returns all the components required to build the overlaid image, but is useful if you only want the Detections.
func (s *Source) NextResult(ctx context.Context) (*Result, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.cancelCtx.Done():
		return nil, s.cancelCtx.Err()
	case result := <-s.pipelineOutput:
		return result, result.Err
	}
}
