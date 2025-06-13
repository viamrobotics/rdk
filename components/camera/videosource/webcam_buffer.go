package videosource

import (
	"context"
	"image"
	"sync"
	"time"

	"github.com/pion/mediadevices/pkg/io/video"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/logging"
)

const sizeOfBuffer = 1

// WebcamBuffer is a buffer for webcam frames.
type WebcamBuffer struct {
	frames           []image.Image  // Holds the frames in the buffer
	mu               sync.RWMutex   // Mutex to synchronize access to the buffer
	currentIndex     int            // Index of the current frame we are accessing
	stopChan         chan struct{}  // Channel to stop the buffer collection process
	currentlyRunning bool           // Checks if the buffer collection process is running
	reader           video.Reader   // Reader to read frames from the webcam
	logger           logging.Logger // Logger for errors or debugging
	workers          *goutils.StoppableWorkers
	frameRate        float32 // Frame rate in frames per second
}

// NewWebcamBuffer creates a new WebcamBuffer struct, for now, we make the buffer a size of 50 frames.
func NewWebcamBuffer(reader video.Reader, logger logging.Logger, frameRate float32) *WebcamBuffer {
	return &WebcamBuffer{
		frames:       make([]image.Image, sizeOfBuffer),
		currentIndex: 0,
		stopChan:     make(chan struct{}),
		reader:       reader,
		logger:       logger,
		frameRate:    frameRate,
	}
}

// StartBuffer initiates the buffer collection process.
func (wb *WebcamBuffer) StartBuffer() {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wb.currentlyRunning {
		wb.mu.Unlock()
		return
	}

	wb.currentlyRunning = true

	// Calculate ticker duration based on frame rate
	duration := time.Duration(float64(time.Second) / float64(wb.frameRate))
	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	wb.workers.Add(func(ctx context.Context) {
		for {
			select {
			case <-wb.stopChan:
				return
			case <-ticker.C:
				img, release, err := wb.reader.Read()
				if err != nil {
					wb.logger.Errorf("error reading frame: %v", err)
					continue
				}

				wb.mu.Lock()
				wb.frames[wb.currentIndex] = img
				wb.currentIndex = (wb.currentIndex + 1) % sizeOfBuffer
				wb.mu.Unlock()

				if release != nil {
					release()
				}
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
		wb.currentlyRunning = false
	}
}

// GetLatestFrame gets the latest frame from the buffer.
func (wb *WebcamBuffer) GetLatestFrame() image.Image {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wb.currentIndex == 0 {
		return wb.frames[sizeOfBuffer-1]
	}

	return wb.frames[wb.currentIndex-1]
}
