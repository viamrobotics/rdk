package data

import (
	"time"

	"github.com/benbjohnson/clock"
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/anypb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/datamanager/datacapture"
)

// CollectorConstructor contains a function for constructing an instance of a Collector.
type CollectorConstructor func(resource interface{}, params CollectorParams) (Collector, error)

// CollectorParams contain the parameters needed to construct a Collector.
type CollectorParams struct {
	ComponentName string
	Interval      time.Duration
	MethodParams  map[string]*anypb.Any
	Target        datacapture.BufferedWriter
	QueueSize     int
	BufferSize    int
	Logger        logging.Logger
	Clock         clock.Clock
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
	return nil
}

// MethodMetadata contains the metadata identifying a component method that we are going to capture and collect.
type MethodMetadata struct {
	API        resource.API
	MethodName string
}

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
func CollectorLookup(method MethodMetadata) *CollectorConstructor {
	if registration, ok := RegisteredCollectors()[method]; ok {
		return &registration
	}
	return nil
}

// RegisteredCollectors returns a copy of the registry.
func RegisteredCollectors() map[MethodMetadata]CollectorConstructor {
	copied, err := copystructure.Copy(collectorRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[MethodMetadata]CollectorConstructor)
}
