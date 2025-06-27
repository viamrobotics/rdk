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
