package datacapture

import (
	"sync"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
)

var (
	// ErrQueueClosed indicates that a Push or Pop was attempted on a closed queue.
	ErrQueueClosed = errors.New("queue is closed")
	MaxSize        = int64(4096)
)

// Queue is a persistent queue of SensorData backed by a series of datacapture.Files.
type Queue struct {
	Directory string
	MetaData  *v1.DataCaptureMetadata
	// TODO: should this just be a byte array that we only write when new Next is assigned?
	nextFile *File
	lock     *sync.Mutex
	files    []*File
	closed   bool
}

func NewQueue(dir string, md *v1.DataCaptureMetadata) *Queue {
	return &Queue{
		Directory: dir,
		files:     []*File{},
		lock:      &sync.Mutex{},
		MetaData:  md,
	}
}

func (d *Queue) Push(item *v1.SensorData) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.closed {
		return ErrQueueClosed
	}

	if d.nextFile == nil {
		nextFile, err := NewFile(d.Directory, d.MetaData)
		if err != nil {
			return err
		}
		d.nextFile = nextFile
	} else if d.nextFile.Size() > MaxSize || item.GetBinary() != nil {
		// If nextFile is >MAX_SIZE or it's a binary reading, update nextFile.
		if err := d.sync(); err != nil {
			return err
		}
		nextFile, err := NewFile(d.Directory, d.MetaData)
		if err != nil {
			return err
		}
		d.nextFile = nextFile
	}

	return d.nextFile.WriteNext(item)
}

// TODO: return err.
// TODO: should probably not return err queue closed? We need to be able to drain queue after closing
func (d *Queue) Pop() (*File, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.closed {
		return nil, ErrQueueClosed
	}

	// Always push nextFile to queue on Pop.
	if err := d.sync(); err != nil {
		return nil, err
	}

	// If files queue is empty, return next file.
	if len(d.files) == 0 {
		// TODO: this feel unidiomatic to return nil, nil
		return nil, nil
	}

	// else, return the next file in the queue, and update the queue
	ret := d.files[0]

	if len(d.files) == 1 {
		d.files = []*File{}
	} else {
		d.files = d.files[1:]
	}
	return ret, nil
}

func (d *Queue) Close() error {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.closed = true
	if d.nextFile == nil {
		return nil
	}
	if err := d.nextFile.Sync(); err != nil {
		return err
	}
	d.nextFile = nil
	return nil
}

func (d *Queue) IsClosed() bool {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.closed
}

func (d *Queue) sync() error {
	if d.nextFile == nil {
		return nil
	}
	if err := d.nextFile.Sync(); err != nil {
		return err
	}
	// if err := d.nextFile.Close(); err != nil {
	//	return err
	//}
	//
	//f, err := os.Open(d.nextFile.file.Name())
	//if err != nil {
	//	return err
	//}
	//endOfQueue, err := ReadFile(f)
	//if err != nil {
	//	return err
	//}
	d.files = append(d.files, d.nextFile)
	d.nextFile = nil
	return nil
}
