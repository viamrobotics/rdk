package data

import (
	"fmt"
	"maps"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// CollectorConstructor contains a function for constructing an instance of a Collector.
type CollectorConstructor func(resource interface{}, params CollectorParams) (Collector, error)

// CollectorParams contain the parameters needed to construct a Collector.
type CollectorParams struct {
	BufferSize      int
	Clock           clock.Clock
	ComponentName   string
	ComponentType   string
	DataType        CaptureType
	Interval        time.Duration
	Logger          logging.Logger
	MethodName      string
	MethodParams    map[string]*anypb.Any
	MongoCollection *mongo.Collection
	QueueSize       int
	Target          CaptureBufferedWriter
}

// Validate validates that p contains all required parameters.
func (p CollectorParams) Validate() error {
	if p.Target == nil {
		return errors.New("missing required parameter target")
	}
	if p.Logger == nil {
		return errors.New("missing required parameter logger")
	}
	if p.ComponentName == "" {
		return errors.New("missing required parameter component name")
	}
	if p.DataType != CaptureTypeBinary && p.DataType != CaptureTypeTabular {
		return errors.New("invalid DataType")
	}
	return nil
}

// MethodMetadata contains the metadata identifying a component method that we are going to capture and collect.
type MethodMetadata struct {
	API        resource.API
	MethodName string
}

func (m MethodMetadata) String() string {
	return fmt.Sprintf("Api: %v, Method Name: %s", m.API, m.MethodName)
}

// collectorRegistry is accessed without locks. This is safe because all collectors are registered
// in package initialization functions. Those functions are executed in series.
var collectorRegistry = map[MethodMetadata]CollectorConstructor{}

// RegisterCollector registers a Collector to its corresponding MethodMetadata.
func RegisterCollector(method MethodMetadata, c CollectorConstructor) {
	_, old := collectorRegistry[method]
	if old {
		panic(errors.Errorf("trying to register two of the same method on the same component: "+
			"component %s, method %s", method.API, method.MethodName))
	}
	collectorRegistry[method] = c
}

// CollectorLookup looks up a Collector by the given MethodMetadata. nil is returned if
// there is None.
func CollectorLookup(method MethodMetadata) CollectorConstructor {
	return collectorRegistry[method]
}

// DumpRegisteredCollectors returns all registered collectores
// this is only intended for services/datamanager/builtin/builtin_test.go.
func DumpRegisteredCollectors() map[MethodMetadata]CollectorConstructor {
	return maps.Clone(collectorRegistry)
}
