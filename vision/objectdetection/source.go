package objectdetection

import (
	"context"
	"image"
	"sync"
	"time"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
)

// Source "pulls" an image from src and applies the detector pipeline to it, resulting in an image overlaid with detections.
// Fulfills gostream.ImageSource interface.
type Source struct {
	src                     gostream.ImageSource
	imageInput, imageOutput chan *Result
	updater                 chan struct{}
	cache                   *Result
	ticker                  *time.Ticker
	mutex                   sync.RWMutex
}

// Result all useful information for the detector: contains the original image, the preprocessed image, and the final detections.
type Result struct {
	OriginalImage     image.Image
	PreprocessedImage image.Image
	Detections        []Detection
	Err               error
}

func buildAndStartPipeline(prep Preprocessor, det Detector, filt Filter) (chan *Result, chan *Result) {
	// define the pipeline functions
	copyImage := func(in <-chan *Result, out chan<- *Result) {
		for r := range in {
			r.PreprocessedImage = CopyImage(r.OriginalImage)
			out <- r
		}
		close(out)
	}
	preprocess := func(in <-chan *Result, out chan<- *Result) {
		for r := range in {
			if r.Err == nil {
				r.PreprocessedImage = prep(r.PreprocessedImage)
			}
			out <- r
		}
		close(out)
	}
	detection := func(in <-chan *Result, out chan<- *Result) {
		for r := range in {
			if r.Err == nil {
				r.Detections, r.Err = det(r.PreprocessedImage)
			}
			out <- r
		}
		close(out)
	}
	filter := func(in <-chan *Result, out chan<- *Result) {
		for r := range in {
			if r.Err == nil {
				r.Detections = filt(r.Detections)
			}
			out <- r
		}
		close(out)
	}
	// create the channels and run
	images := make(chan *Result)
	copied := make(chan *Result)
	processed := make(chan *Result)
	detected := make(chan *Result)
	filtered := make(chan *Result)
	go copyImage(images, copied)
	go preprocess(copied, processed)
	go detection(processed, detected)
	go filter(detected, filtered)
	return images, filtered
}

// NewSource builds the pipeline from an input ImageSource, Preprocessor (optional), Detector (required) and  Filter (optional).
func NewSource(src gostream.ImageSource, prep Preprocessor, det Detector, filt Filter, fps float64) (*Source, error) {
	// fill optional functions with identity operators
	if src == nil {
		return nil, errors.New("object detection source must include an image source to pull from")
	}
	if prep == nil {
		prep = func(img image.Image) image.Image { return img }
	}
	if det == nil {
		return nil, errors.New("object detector function cannot be nil")
	}
	if filt == nil {
		filt = func(inp []Detection) []Detection { return inp }
	}
	imageInputChan, imageOutputChan := buildAndStartPipeline(prep, det, filt)
	// variable initialization
	r := &Result{}
	r.OriginalImage, _, r.Err = src.Next(context.Background())
	if r.Err != nil {
		return nil, r.Err
	}
	imageInputChan <- r
	r = <-imageOutputChan
	if r.Err != nil {
		return nil, r.Err
	}
	// return the Source
	updaterChan := make(chan struct{})
	tickTime := int((1. / fps) * 1000.)
	ticker := time.NewTicker(time.Duration(tickTime) * time.Millisecond) 
	s := &Source{
		src:         src,
		imageInput:  imageInputChan,
		imageOutput: imageOutputChan,
		updater:     updaterChan,
		cache:       r,
		ticker:      ticker,
		mutex:       sync.RWMutex{},
	}
	go s.startUpdater()
	return s, nil
}

// Close closes all the channels and threads.
func (s *Source) Close() {
	s.ticker.Stop()
	s.updater <- struct{}{}
	close(s.updater)
	close(s.imageInput)
}

// startUpdater is running in background and updates detections on the ticker.
func (s *Source) startUpdater() {
	for {
		select {
		case <-s.ticker.C:
			// update cache
			r := s.runPipeline()
			s.mutex.Lock() // lock the cache before writing into it
			s.cache = r
			s.mutex.Unlock() // unlock the cache before writing into it
		case <-s.updater:
			// stop the updater
			return
		}
	}
}

func (s *Source) runPipeline() *Result {
	r := &Result{}
	r.OriginalImage, _, r.Err = s.src.Next(context.Background())
	s.imageInput <- r
	r = <-s.imageOutput
	return r
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
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	r := s.cache
	return r, r.Err
}
