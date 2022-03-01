package collector

import (
	"github.com/golang/protobuf/ptypes/any"
	"os"
	"reflect"
	"sync"
)

type Metadata struct {
	Component string
	Method    string
	// TODO: should this be an absolute path instead of a File?
	Destination *os.File
	Params      []string
}

type Collector struct {
	lock     *sync.Mutex
	metadata Metadata
	client   interface{} // TODO: find more specific generic interface for a PB service client if one exists
	queue    chan []byte // TODO: Will actually be chan of SensorData; just putting any.Any because I want this to compile.
}

func NewCollector(md Metadata, client interface{}) (Collector, error) {
	c := Collector{
		lock:     &sync.Mutex{},
		metadata: md,
		client:   client,
		queue:    make(chan []byte),
	}
	err := c.validate()
	if err != nil {
		// TODO: return appropriate error
		return c, nil
	}

	return c, nil
}

// TODO: validate. Check that method exists. Maybe check that params are correct.
func (c Collector) validate() error {
	return nil
}

func (c Collector) Collect(periodMS int64) error {
	for {
		// TODO: pass params
		raw := reflect.ValueOf(&c).MethodByName(c.metadata.Method).Call([]reflect.Value{})
		reading, ok := reflect.ValueOf(raw).Interface().(any.Any)
		if !ok {
			// TODO: return some appropriate error
			return nil
		}
		c.queue <- reading.Value
	}

	return nil
}

func (c Collector) Write() error {
	for r := range c.queue {
		c.Lock()
		err := bufferedWrite(c.metadata.Destination, r)
		c.Unlock()
		if err != nil {
			return err
		}
	}

	return nil
}

// TODO: implement buffered write method that appends a length-prefixed proto message to a file, as desc in tech spec
func bufferedWrite(f *os.File, reading []byte) error {
	return nil
}

func (c Collector) Lock() {
	c.lock.Lock()
}

func (c Collector) Unlock() {
	c.lock.Unlock()
}

func (c Collector) SetMetadata(md Metadata) {
	c.Lock()
	c.metadata = md
	c.Unlock()
}

func (c Collector) GetMetadata() Metadata {
	return c.metadata
}
