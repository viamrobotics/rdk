package datacapture

import (
	v1 "go.viam.com/api/app/datasync/v1"
	"sync"
)

const (
	maxSize = 4096
)

type Deque struct {
	directory string
	md        *v1.DataCaptureMetadata
	nextFile  *File
	lock      *sync.Mutex
	files     []*File
}

func (d *Deque) NewDeque(dir string) *Deque {
	return &Deque{
		directory: dir,
		files:     []*File{},
	}
}

func (d *Deque) Enqueue(item *v1.SensorData) error {
	if d.nextFile == nil {
		nextFile, err := NewFile(d.directory, d.md)
		if err != nil {
			return err
		}
		d.nextFile = nextFile
	}

	// If nextFile is >MAX_SIZE, update nextFile.
	if d.nextFile.Size() > maxSize {
		if err := d.nextFile.Sync(); err != nil {
			return err
		}
		d.lock.Lock()
		d.files = append(d.files, d.nextFile)
		nextFile, err := NewFile(d.directory, d.md)
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
	if len(d.files) == 0 {
		return nil
	}
	ret := d.files[0]
	d.files = d.files[1:]
	d.lock.Unlock()
	return ret
}
