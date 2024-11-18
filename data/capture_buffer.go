package data

import (
	"sync"

	v1 "go.viam.com/api/app/datasync/v1"
)

const captureAllFromCamera = "CaptureAllFromCamera"

// CaptureBufferedWriter is a buffered, persistent queue of SensorData.
type CaptureBufferedWriter interface {
	Write(item *v1.SensorData) error
	Flush() error
	Path() string
}

// CaptureBuffer is a persistent queue of SensorData backed by a series of *data.CaptureFile.
type CaptureBuffer struct {
	Directory          string
	MetaData           *v1.DataCaptureMetadata
	nextFile           *CaptureFile
	lock               sync.Mutex
	maxCaptureFileSize int64
}

// NewCaptureBuffer returns a new Buffer.
func NewCaptureBuffer(dir string, md *v1.DataCaptureMetadata, maxCaptureFileSize int64) *CaptureBuffer {
	return &CaptureBuffer{
		Directory:          dir,
		MetaData:           md,
		maxCaptureFileSize: maxCaptureFileSize,
	}
}

// Write writes item onto b. Binary sensor data is written to its own file.
// Tabular data is written to disk in maxCaptureFileSize sized files. Files that
// are still being written to are indicated with the extension
// InProgressFileExt. Files that have finished being written to are indicated by
// FileExt.
func (b *CaptureBuffer) Write(item *v1.SensorData) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if item.GetBinary() != nil {
		binFile, err := NewCaptureFile(b.Directory, b.MetaData)
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
		nextFile, err := NewCaptureFile(b.Directory, b.MetaData)
		if err != nil {
			return err
		}
		b.nextFile = nextFile
		// We want to special case on "CaptureAllFromCamera" because it is sensor data that contains images
		// and their corresponding annotations. We want each image and its annotations to be stored in a
		// separate file.
	} else if b.nextFile.Size() > b.maxCaptureFileSize || b.MetaData.MethodName == captureAllFromCamera {
		if err := b.nextFile.Close(); err != nil {
			return err
		}
		nextFile, err := NewCaptureFile(b.Directory, b.MetaData)
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
	return b.Directory
}
