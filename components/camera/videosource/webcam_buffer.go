package videosource

import (
	"image"
	"sync"

	"github.com/pion/mediadevices/pkg/io/video"
	"go.viam.com/rdk/logging"
)

type WebcamBuffer struct {
	frames        	[]image.Image
	sizeOfBuffer    int
	mu            	sync.RWMutex
	currentIndex    int
	stopChan 		chan struct{}
	currentlyRunning 	bool
	reader 			video.Reader
	logger 			logging.Logger
}

// Constructs a new WebcameBuffer struct, for now, we make the buffer a size of 50 frames
func NewWebcamBuffer(reader video.Reader, logger logging.Logger) *WebcamBuffer {
	return &WebcamBuffer{
		frames: make([]image.Image, 50),
		sizeOfBuffer:   50,
		currentIndex: 0,
		stopChan: make(chan struct{}),
		reader: reader,
		logger: logger,
	}
}

func (wb *WebcamBuffer) Start() {
	wb.mu.Lock()

	if wb.currentlyRunning {
		wb.mu.Unlock()
		return
	}

	wb.currentlyRunning = true
	wb.mu.Unlock()
	go wb.CollectFrames()
}

func (wb *WebcamBuffer) Stop() {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if !wb.currentlyRunning {
		return
	}

	close(wb.stopChan)
	wb.currentlyRunning = false
}

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

// Get the latest frame from the buffer
func (wb *WebcamBuffer) GetLatestFrame() image.Image {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wb.currentIndex == 0 {
		return wb.frames[wb.sizeOfBuffer - 1]
	}

	return wb.frames[wb.currentIndex - 1]
}
