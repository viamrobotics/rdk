package datacapture

import (
	"sync"

	v1 "go.viam.com/api/app/datasync/v1"
)

// MaxFileSize is the maximum size in bytes of a data capture file.
var MaxFileSize = int64(64 * 1024)

// BufferedWriter is a buffered, persistent queue of SensorData.
type BufferedWriter interface {
	Write(item *v1.SensorData) error
	Flush() error
	Path() string
}

// Buffer is a persistent queue of SensorData backed by a series of datacapture.Files.
type Buffer struct {
	Directory string
	MetaData  *v1.DataCaptureMetadata
	nextFile  *File
	lock      sync.Mutex
}

// NewBuffer returns a new Buffer.
func NewBuffer(dir string, md *v1.DataCaptureMetadata) *Buffer {
	return &Buffer{
		Directory: dir,
		MetaData:  md,
	}
}

// Write writes item onto b. Binary sensor data is written to its own file.
// Tabular data is written to disk in MaxFileSize sized files. Files that are still being written to are indicated
// with the extension InProgressFileExt. Files that have finished being written to are indicated by FileExt.
func (b *Buffer) Write(item *v1.SensorData) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if item.GetBinary() != nil {
		binFile, err := NewFile(b.Directory, b.MetaData)
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
		nextFile, err := NewFile(b.Directory, b.MetaData)
		if err != nil {
			return err
		}
		b.nextFile = nextFile
	} else if b.nextFile.Size() > MaxFileSize {
		if err := b.nextFile.Close(); err != nil {
			return err
		}
		nextFile, err := NewFile(b.Directory, b.MetaData)
		if err != nil {
			return err
		}
		b.nextFile = nextFile
	}

	return b.nextFile.WriteNext(item)
}

// Flush flushes all buffered data to disk and marks any in progress file as complete.
func (b *Buffer) Flush() error {
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
func (b *Buffer) Path() string {
	return b.Directory
}
