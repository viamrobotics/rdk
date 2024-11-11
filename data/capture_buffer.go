package data

import (
	"sync"

	v1 "go.viam.com/api/app/datasync/v1"
)

// CaptureBufferedWriter is a buffered, persistent queue of SensorData.
type CaptureBufferedWriter interface {
	Write(item *v1.SensorData) error
	Flush() error
	Path() string
}

// CaptureBuffer is a persistent queue of SensorData backed by a series of *data.CaptureFile.
type CaptureBuffer struct {
	directory          string
	metaData           *v1.DataCaptureMetadata
	nextFile           *ProgFile
	lock               sync.Mutex
	maxCaptureFileSize int64
}

// NewCaptureBuffer returns a new Buffer.
func NewCaptureBuffer(dir string, md *v1.DataCaptureMetadata, maxCaptureFileSize int64) *CaptureBuffer {
	return &CaptureBuffer{
		directory:          dir,
		metaData:           md,
		maxCaptureFileSize: maxCaptureFileSize,
	}
}

// Write writes item onto b. Binary sensor data is written to its own file.
// Tabular data is written to disk in maxCaptureFileSize sized files. Files that
// are still being written to are indicated with the extension
// InProgressFileExt. Files that have finished being written to are indicated by
// FileExt.
func isBinary(item *v1.SensorData) bool {
	if item == nil {
		return false
	}
	switch item.Data.(type) {
	case *v1.SensorData_Binary:
		return true
	default:
		return false
	}
}

func (b *CaptureBuffer) Write(item *v1.SensorData) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if isBinary(item) {
		binFile, err := NewProgFile(b.directory, b.metaData)
		if err != nil {
			return err
		}
		if err := binFile.WriteNext(item); err != nil {
			return err
		}
		if err := binFile.Close(); err != nil {
			return err
		}
		return nil
	}

	if b.nextFile == nil {
		nextFile, err := NewProgFile(b.directory, b.metaData)
		if err != nil {
			return err
		}
		b.nextFile = nextFile
		// We want to special case on "CaptureAllFromCamera" because it is sensor data that contains images
		// and their corresponding annotations. We want each image and its annotations to be stored in a
		// separate file.
	} else if b.nextFile.Size() > b.maxCaptureFileSize || b.metaData.MethodName == "CaptureAllFromCamera" {
		if err := b.nextFile.Close(); err != nil {
			return err
		}
		nextFile, err := NewProgFile(b.directory, b.metaData)
		if err != nil {
			return err
		}
		b.nextFile = nextFile
	}

	return b.nextFile.WriteNext(item)
}

// Flush flushes all buffered data to disk and marks any in progress file as complete.
func (b *CaptureBuffer) Flush() error {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.nextFile == nil {
		return nil
	}
	if err := b.nextFile.Close(); err != nil {
		return err
	}
	b.nextFile = nil
	return nil
}

// Path returns the path to the directory containing the backing data capture files.
func (b *CaptureBuffer) Path() string {
	return b.directory
}
