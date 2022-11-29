package datacapture

import (
	"sync"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
)

var (
	// ErrQueueClosed indicates that a Push or Pop was attempted on a closed queue.
	ErrQueueClosed = errors.New("queue is closed")
	// MaxFileSize is the maximum size in bytes of a data capture file.
	MaxFileSize = int64(65536)
)

// Queue is a persistent queue of SensorData backed by a series of datacapture.Files.
type Queue struct {
	Directory string
	MetaData  *v1.DataCaptureMetadata
	nextFile  *File
	lock      *sync.Mutex
	files     []*File
	closed    bool
}

// NewQueue returns a new Queue.
func NewQueue(dir string, md *v1.DataCaptureMetadata) *Queue {
	return &Queue{
		Directory: dir,
		files:     []*File{},
		lock:      &sync.Mutex{},
		MetaData:  md,
	}
}

// Push pushes item onto q.
func (q *Queue) Push(item *v1.SensorData) error {
	q.lock.Lock()
	defer q.lock.Unlock()

	if q.closed {
		return ErrQueueClosed
	}

	if q.nextFile == nil {
		nextFile, err := NewFile(q.Directory, q.MetaData)
		if err != nil {
			return err
		}
		q.nextFile = nextFile
	} else if q.nextFile.Size() > MaxFileSize || item.GetBinary() != nil {
		// If nextFile is >MAX_SIZE or it's a binary reading, update nextFile.
		if err := q.pushNextFile(); err != nil {
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

// Pop returns the next File in q.
func (q *Queue) Pop() (*File, error) {
	q.lock.Lock()
	defer q.lock.Unlock()

	// Push nextFile to queue on Pop.
	if err := q.pushNextFile(); err != nil {
		return nil, err
	}

	// if queue is empty, return nil.
	//nolint:nilnil
	if len(q.files) == 0 {
		return nil, nil
	}

	// else, return the next file in the queue, and update the queue
	ret := q.files[0]
	q.files = q.files[1:]
	return ret, nil
}

// Close closes the queue, indicating no additional data will be pushed.
func (q *Queue) Close() error {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.closed = true
	if q.nextFile == nil {
		return nil
	}
	if err := q.nextFile.Sync(); err != nil {
		return err
	}
	q.nextFile = nil
	return nil
}

// IsClosed returns whether or not q is closed.
func (q *Queue) IsClosed() bool {
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.closed
}

// Reset removes all items from the queue. It does not delete the underlying files from disk.
func (q *Queue) Reset() {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.nextFile = nil
	q.files = []*File{}
}

func (q *Queue) pushNextFile() error {
	if q.nextFile == nil {
		return nil
	}
	if err := q.nextFile.Sync(); err != nil {
		return err
	}
	q.files = append(q.files, q.nextFile)
	q.nextFile = nil
	return nil
}
