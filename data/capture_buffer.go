package data

import (
	"sync"

	v1 "go.viam.com/api/app/datasync/v1"
)

// CaptureBufferedWriter is a buffered, persistent queue of SensorData.
type CaptureBufferedWriter interface {
	WriteBinary(items []*v1.SensorData) error
	WriteTabular(items []*v1.SensorData) error
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

// WriteBinary writes the items to their own file.
// Files that are still being written to are indicated with the extension
// '.prog'.
// Files that have finished being written to are indicated by
// '.capture'.
func (b *CaptureBuffer) WriteBinary(items []*v1.SensorData) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	binFile, err := NewCaptureFile(b.Directory, b.MetaData)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := binFile.WriteNext(item); err != nil {
			return err
		}
	}
	if err := binFile.Close(); err != nil {
		return err
	}
	return nil
}

// WriteTabular writes
// Tabular data to disk in maxCaptureFileSize sized files.
// Files that are still being written to are indicated with the extension
// '.prog'.
// Files that have finished being written to are indicated by
// '.capture'.
func (b *CaptureBuffer) WriteTabular(items []*v1.SensorData) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.nextFile == nil {
		nextFile, err := NewCaptureFile(b.Directory, b.MetaData)
		if err != nil {
			return err
		}
		b.nextFile = nextFile
	} else if b.nextFile.Size() > b.maxCaptureFileSize {
		if err := b.nextFile.Close(); err != nil {
			return err
		}
		nextFile, err := NewCaptureFile(b.Directory, b.MetaData)
		if err != nil {
			return err
		}
		b.nextFile = nextFile
	}

	for _, item := range items {
		if err := b.nextFile.WriteNext(item); err != nil {
			return err
		}
	}

	return nil
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
