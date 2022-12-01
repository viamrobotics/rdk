package datacapture

import (
	"sync"

	v1 "go.viam.com/api/app/datasync/v1"
)

// MaxFileSize is the maximum size in bytes of a data capture file.
var MaxFileSize = int64(64 * 1024)

// Queue is a persistent queue of SensorData backed by a series of datacapture.Files.
type Queue struct {
	Directory string
	MetaData  *v1.DataCaptureMetadata
	nextFile  *File
	lock      *sync.Mutex
}

// NewQueue returns a new Queue.
func NewQueue(dir string, md *v1.DataCaptureMetadata) *Queue {
	return &Queue{
		Directory: dir,
		lock:      &sync.Mutex{},
		MetaData:  md,
	}
}

// Push pushes item onto q.
func (q *Queue) Push(item *v1.SensorData) error {
	q.lock.Lock()
	defer q.lock.Unlock()

	if item.GetBinary() != nil {
		binFile, err := NewFile(q.Directory, q.MetaData)
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

	if q.nextFile == nil {
		nextFile, err := NewFile(q.Directory, q.MetaData)
		if err != nil {
			return err
		}
		q.nextFile = nextFile
	} else if q.nextFile.Size() > MaxFileSize {
		// If nextFile is >MAX_SIZE or it's a binary reading, update nextFile.
		if err := q.nextFile.Close(); err != nil {
			return err
		}
		nextFile, err := NewFile(q.Directory, q.MetaData)
		if err != nil {
			return err
		}
		q.nextFile = nextFile
	}

	return q.nextFile.WriteNext(item)
}

// Sync flushes all buffered data to disk and marks any in progress file as complete.
func (q *Queue) Sync() error {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.nextFile == nil {
		return nil
	}
	if err := q.nextFile.Close(); err != nil {
		return err
	}
	q.nextFile = nil
	return nil
}
