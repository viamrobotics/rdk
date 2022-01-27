package objectdetection

import (
	"context"
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
)

// Source "pulls" an image from src and applies the detector pipeline to it, resulting in an image overlaid with detections.
// Fulfills gostream.ImageSource interface.
type Source struct {
	src                      gostream.ImageSource
	imageInput, imageOutput  chan *Result
	counter                  chan float64
	stopUpdater, stopCounter chan struct{}
	cache                    *Result
	ticker                   *time.Ticker
	mutex                    sync.RWMutex
	actualFps                float64
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

func buildSerialPipeline(prep Preprocessor, det Detector, filt Filter) (chan *Result, chan *Result) {
	// define the full pipeline
	pipeline := func(in <-chan *Result, out chan<- *Result) {
		for r := range in {
			r.PreprocessedImage = CopyImage(r.OriginalImage)
			if r.Err == nil {
				r.PreprocessedImage = prep(r.PreprocessedImage)
			}
			if r.Err == nil {
				r.Detections, r.Err = det(r.PreprocessedImage)
			}
			if r.Err == nil {
				r.Detections = filt(r.Detections)
			}
			out <- r
		}
		close(out)
	}
	// create the channels and run
	imageIn := make(chan *Result)
	imageOut := make(chan *Result)
	go pipeline(imageIn, imageOut)
	return imageIn, imageOut
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
	//imageInputChan, imageOutputChan := buildSerialPipeline(prep, det, filt)
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
	tickTime := int((1. / fps) * 1000.)
	ticker := time.NewTicker(time.Duration(tickTime) * time.Millisecond)
	s := &Source{
		src:         src,
		imageInput:  imageInputChan,
		imageOutput: imageOutputChan,
		counter:     make(chan float64),
		stopUpdater: make(chan struct{}),
		stopCounter: make(chan struct{}),
		cache:       r,
		ticker:      ticker,
		mutex:       sync.RWMutex{},
		actualFps:   0.,
	}
	go s.startUpdater()
	go s.startFpsCounter()
	return s, nil
}

// Close closes all the channels and threads.
func (s *Source) Close() {
	s.ticker.Stop()
	s.stopUpdater <- struct{}{}
	close(s.stopUpdater)
	s.stopCounter <- struct{}{}
	close(s.stopCounter)
	close(s.imageInput)
	close(s.counter)
}

// startFpsCounter is running in the background and counts how many times frames have been returned in a second
func (s *Source) startFpsCounter() {
	i := 0.0
	for start := time.Now(); ; {
		if time.Since(start) > 5*time.Second {
			s.mutex.Lock()
			s.actualFps = i / 5.0
			s.mutex.Unlock()
			i = 0.0
			start = time.Now()
		}
		select {
		case j := <-s.counter:
			i += j
		case <-s.stopCounter:
			return
		default:
			// do nothing.
		}
	}

}

// startUpdater is running in background and updates detections on the ticker.
func (s *Source) startUpdater() {
	for {
		select {
		case <-s.ticker.C:
			r := s.runPipeline(context.Background())
			s.mutex.Lock() // lock the cache before writing into it
			s.cache = r
			s.mutex.Unlock()
		case <-s.stopUpdater:
			// stop the updater
			return
		}
	}
}

func (s *Source) runPipeline(ctx context.Context) *Result {
	r := &Result{}
	r.OriginalImage, _, r.Err = s.src.Next(ctx)
	s.imageInput <- r
	r = <-s.imageOutput
	s.counter <- 1.0
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
	ovImg = OverlayText(ovImg, fmt.Sprintf("FPS: %.2f", s.FPS()))
	return ovImg, func() {}, nil
}

// NextResult returns all the components required to build the overlaid image, but is useful if you only want the Detections.
func (s *Source) NextResult(ctx context.Context) (*Result, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	r := s.cache
	//r := s.runPipeline(ctx)
	return r, r.Err
}

func (s *Source) FPS() float64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.actualFps
}
