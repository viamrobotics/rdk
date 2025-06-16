package videosource

import (
	"context"
	"errors"
	"image"
	"sync"
	"time"

	"github.com/pion/mediadevices/pkg/io/video"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/logging"
)

const sizeOfBuffer = 2 // We only need 2 frames in the buffer because we are only ever using the latest frame.

type frameStruct struct {
	img     image.Image
	release func()
	err     error
}

// WebcamBuffer is a buffer for webcam frames.
type WebcamBuffer struct {
	frames       []frameStruct // Holds the frames and their release functions in the buffer
	mu           sync.RWMutex  // Mutex to synchronize access to the buffer
	currentIndex int           // Index of the current frame we are accessing
	reader       video.Reader
	logger       logging.Logger
	workers      *goutils.StoppableWorkers
	frameRate    float32      // Frame rate in frames per second
	ticker       *time.Ticker // Ticker for controlling frame rate
}

// NewWebcamBuffer creates a new WebcamBuffer struct.
func NewWebcamBuffer(reader video.Reader, logger logging.Logger, frameRate float32) *WebcamBuffer {
	return &WebcamBuffer{
		frames:       make([]frameStruct, sizeOfBuffer),
		currentIndex: 0,
		reader:       reader,
		logger:       logger,
		frameRate:    frameRate,
		workers:      goutils.NewBackgroundStoppableWorkers(),
	}
}

// StartBuffer initiates the buffer collection process.
func (wb *WebcamBuffer) StartBuffer() {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wb.ticker != nil {
		return
	}

	interFrameDuration := time.Duration(float32(time.Second) / wb.frameRate)

	wb.workers.Add(func(closedCtx context.Context) {
		wb.ticker = time.NewTicker(interFrameDuration)
		defer wb.ticker.Stop()

		for {
			select {
			case <-closedCtx.Done():
				return
			case <-wb.ticker.C:
				img, release, err := wb.reader.Read()
				if err != nil {
					wb.logger.Errorf("error reading frame: %v", err)
					continue
				}

				wb.mu.Lock()
				if wb.frames[wb.currentIndex].release != nil {
					wb.frames[wb.currentIndex].release()
				}

				wb.frames[wb.currentIndex] = frameStruct{
					img:     img,
					release: release,
					err:     err,
				}
				wb.currentIndex = (wb.currentIndex + 1) % sizeOfBuffer
				wb.mu.Unlock()
			}
		}
	})
}

// StopBuffer stops the buffer collection process.
func (wb *WebcamBuffer) StopBuffer() {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wb.workers != nil {
		wb.workers.Stop()
	}

	if wb.ticker != nil {
		wb.ticker.Stop()
		wb.ticker = nil
	}

	// Release any remaining frames.
	for i := range wb.frames {
		if wb.frames[i].release != nil {
			wb.frames[i].release()
			wb.frames[i].release = nil
		}
	}
}

// GetLatestFrame gets the latest frame from the buffer.
func (wb *WebcamBuffer) GetLatestFrame() (image.Image, error) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	var latestFrame frameStruct
	if wb.currentIndex == 0 {
		latestFrame = wb.frames[sizeOfBuffer-1]
	} else {
		latestFrame = wb.frames[wb.currentIndex-1]
	}

	if latestFrame.img == nil {
		if latestFrame.err != nil {
			return nil, latestFrame.err
		}
		return nil, errors.New("no frames available to read")
	}

	return latestFrame.img, nil
}
