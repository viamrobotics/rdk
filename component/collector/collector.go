package collector

import (
	"github.com/golang/protobuf/ptypes/any"
	"os"
	"reflect"
	"sync"
)

type Metadata struct {
	component string
	method    string
	// TODO: should this be an absolute path instead of a File?
	destination *os.File
	params      map[string]string
}

type Collector struct {
	lock     *sync.Mutex
	metadata Metadata
	client   interface{} // TODO: find more specific generic interface for a PB service client if one exists
	queue    chan []byte // TODO: Will actually be chan of SensorData; just putting any.Any because I want this to compile.
}

func (c Collector) Collect(periodMS int64) error {
	for {
		// TODO: pass params
		raw := reflect.ValueOf(&c).MethodByName(c.metadata.method).Call([]reflect.Value{})
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
		err := bufferedWrite(c.metadata.destination, r)
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

//type Collector interface {
//	Lock() error
//	// periodMs is period in ms at which data is captured.
//	Collect(periodMs int64) error
//	SetMetadata(md Metadata) error
//	GetMetadata() Metadata
//}
//
//type RotVelocityCollector struct {
//	lock       *sync.Mutex
//	metadata   Metadata
//	params     map[string]string
//	client     pb.IMUServiceClient // TODO: find generic interface for a PB service client
//	methodName string
//	// Will actually be chan of SensorData; just putting any.Any because I want this to compile.
//	queue chan any.Any
//}
//
//// TODO: Add MD and Client as constructor params
//func NewRotVelocityCollector() Collector {
//	md := Metadata{
//		component:   "IMU",
//		method:      "ReadAngularVelocity",
//		destination: nil,
//	}
//
//	ret := RotVelocityCollector{
//		lock:       &sync.Mutex{},
//		metadata:   md,
//		params:     nil,
//		client:     pb.NewIMUServiceClient(nil),
//		methodName: "ReadAngularVelocity",
//		queue:      make(chan any.Any),
//	}
//
//	return ret
//}
//
//func (c RotVelocityCollector) Collect(periodMS int64) error {
//	for {
//		// TODO: pass params
//		reflect.ValueOf(&c).MethodByName(c.methodName).Call([]reflect.Value{})
//	}
//
//	return nil
//}
//
//func (c RotVelocityCollector) Lock() error {
//	return nil
//}
//
//func (c RotVelocityCollector) SetMetadata(md Metadata) error {
//	return nil
//}
//
//func (c RotVelocityCollector) GetMetadata() Metadata {
//	return c.metadata
//}
