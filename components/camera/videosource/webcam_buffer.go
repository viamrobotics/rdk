

const sizeOfBuffer = 2 // We only need 2 frames in the buffer because we are only ever using the latest frame.


// WebcamBuffer is a buffer for webcam frames.
type WebcamBuffer struct {
	frames       []FrameStruct // Holds the frames and their release functions in the buffer
	mu           sync.RWMutex  // Mutex to synchronize access to the buffer frames
	currentIndex int           // Index of the current frame we are accessing
	frameRate    float32       // Frame rate in frames per second
	ticker       *time.Ticker  // Ticker for controlling frame rate
}

// NewWebcamBuffer creates a new WebcamBuffer struct.
func NewWebcamBuffer(frameRate float32) *WebcamBuffer {
	return &WebcamBuffer{
		frames:       make([]FrameStruct, sizeOfBuffer),
		currentIndex: 0,
		frameRate:    frameRate,
	}
}

// GetLatestFrame gets the latest frame from the buffer.
func (wb *WebcamBuffer) GetLatestFrame() (image.Image, error) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	var latestFrame FrameStruct
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
