package videosource

import (
	"image"
	"sync"

	"github.com/pion/mediadevices/pkg/io/video"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

const SIZEOFBUFFER = 1

// WebcamBuffer is a buffer for webcam frames.
type WebcamBuffer struct {
	frames           []image.Image	// Holds the frames in the buffer
	mu               sync.RWMutex	// Mutex to synchronize access to the buffer
	currentIndex     int			// Index of the current frame we are accessing
	stopChan         chan struct{}	// Channel to stop the buffer collection process
	currentlyRunning bool 			// Checks if the buffer collection process is running
	reader           video.Reader	// Reader to read frames from the webcam
	logger           logging.Logger	// Logger for errors or debugging
}

// NewWebcamBuffer creates a new WebcamBuffer struct, for now, we make the buffer a size of 50 frames.
func NewWebcamBuffer(reader video.Reader, logger logging.Logger) *WebcamBuffer {
	return &WebcamBuffer{
		frames:       make([]image.Image, SIZEOFBUFFER),
		currentIndex: 0,
		stopChan:     make(chan struct{}),
		reader:       reader,
		logger:       logger,
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
	go wb.CollectFrames()
}

// StopBuffer stops the buffer collection process.
func (wb *WebcamBuffer) StopBuffer() {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if !wb.currentlyRunning {
		return
	}

	close(wb.stopChan)
	wb.currentlyRunning = false
}

// CollectFrames collects frames from the reader and adds them to the buffer. This function is called in a goroutine.
func (wb *WebcamBuffer) CollectFrames() {
	for {
		select {
		case <-wb.stopChan:
			return
		default:
			img, release, err := wb.reader.Read()
			if err != nil {
				wb.logger.Errorf("error reading frame: %v", err)
				continue
			}

			wb.mu.Lock()
			wb.frames[wb.currentIndex] = img
			wb.currentIndex = (wb.currentIndex + 1) % wb.sizeOfBuffer
			wb.mu.Unlock()

			if release != nil {
				release()
			}
		}
	}
}

// GetLatestFrame gets the latest frame from the buffer.
func (wb *WebcamBuffer) GetLatestFrame() image.Image {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wb.currentIndex == 0 {
		return wb.frames[wb.sizeOfBuffer-1]
	}

	return wb.frames[wb.currentIndex-1]
}
