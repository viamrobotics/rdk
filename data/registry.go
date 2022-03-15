package data

import (
	"os"
	"time"

	"github.com/edaniels/golog"
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/resource"
)

// CollectorConstructor contains a function for constructing an instance of a Collector.
type CollectorConstructor func(conn rpc.ClientConn, params map[string]string, interval time.Duration,
	target *os.File, logger golog.Logger) Collector

// MethodMetadata contains the metadata identifying a component method that we are going to capture and collect.
type MethodMetadata struct {
	Subtype    resource.SubtypeName
	MethodName string
}

var collectorRegistry = map[MethodMetadata]CollectorConstructor{}

// RegisterCollector registers a Collector to its corresponding MethodMetadata.
func RegisterCollector(method MethodMetadata, c CollectorConstructor) {
	_, old := collectorRegistry[method]
	if old {
		panic(errors.Errorf("trying to register two of the same method on the same component: "+
			"component %s, method %s", method.Subtype, method.MethodName))
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
