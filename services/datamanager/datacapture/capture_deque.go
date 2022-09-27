package datacapture

import (
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"path/filepath"
	"sync"
)

const (
	maxSize = 4096
)

var (
	ErrQueueClosed = errors.New("queue is closed")
)

type Deque struct {
	Directory string
	MetaData  *v1.DataCaptureMetadata
	nextFile  *File
	lock      *sync.Mutex
	files     []*File
	closed    bool
}

func NewDeque(dir string, md *v1.DataCaptureMetadata) *Deque {
	queueDir := filepath.Join(dir, getFileTimestampName())
	return &Deque{
		Directory: queueDir,
		files:     []*File{},
		lock:      &sync.Mutex{},
		MetaData:  md,
	}
}

func (d *Deque) Enqueue(item *v1.SensorData) error {
	if d.IsClosed() {
		return ErrQueueClosed
	}

	if d.nextFile == nil {
		nextFile, err := NewFile(d.Directory, d.MetaData)
		if err != nil {
			return err
		}
		d.nextFile = nextFile
	}

	// If nextFile is >MAX_SIZE or it's a binary reading, update nextFile.
	if d.nextFile.Size() > maxSize || item.GetBinary() != nil {
		if err := d.nextFile.Sync(); err != nil {
			return err
		}
		d.lock.Lock()
		d.files = append(d.files, d.nextFile)
		nextFile, err := NewFile(d.Directory, d.MetaData)
		if err != nil {
			d.lock.Unlock()
			return err
		}
		d.nextFile = nextFile
		d.lock.Unlock()
	}

	if err := d.nextFile.WriteNext(item); err != nil {
		return err
	}
	return nil
}

func (d *Deque) Dequeue() *File {
	d.lock.Lock()
	defer d.lock.Unlock()
	if len(d.files) == 0 {
		return nil
	}
	ret := d.files[0]
	d.files = d.files[1:]
	return ret
}

func (d *Deque) Peek() *File {
	d.lock.Lock()
	defer d.lock.Unlock()
	if len(d.files) == 0 {
		return nil
	}
	return d.files[0]
}

func (d *Deque) Close() {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.closed = true
}

func (d *Deque) IsClosed() bool {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.closed
}

func (d *Deque) Sync() error {
	d.lock.Lock()
	defer d.lock.Unlock()
	if err := d.nextFile.Sync(); err != nil {
		return err
	}
	d.files = append(d.files, d.nextFile)
	nextFile, err := NewFile(d.Directory, d.MetaData)
	if err != nil {
		return err
	}
	d.nextFile = nextFile
	return nil
}
