package data

import (
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils/rpc"
	"os"
	"time"
)

type CollectorConstructor struct {
	Constructor func(conn rpc.ClientConn, name string, interval time.Duration, target *os.File) Collector
}

type MethodMetadata struct {
	// TODO: Subtype or SubType?
	Subtype    resource.Subtype
	MethodName string
}

var (
	collectorRegistry = map[MethodMetadata]CollectorConstructor{}
)

// RegisterCollector registers a Collector to its corresponding component subtype.
func RegisterCollector(method MethodMetadata, c CollectorConstructor) {
	_, old := collectorRegistry[method]
	if old {
		panic(errors.Errorf("trying to register two of the same method on the same component: "+
			"component %s, method %s", method.Subtype, method.MethodName))
	}
	if c.Constructor == nil {
		panic(errors.Errorf("cannot register a data collector with a nil constructor"))
	}

	collectorRegistry[method] = c
}

// CollectorLookup looks up a Collector by the given subtype. nil is returned if
// there is None.
func CollectorLookup(method MethodMetadata) *CollectorConstructor {
	if registration, ok := RegisteredCollectors()[method]; ok {
		return &registration
	}
	return nil
}

// RegisteredCollectors returns a copy of the registered CollectorSchema.
func RegisteredCollectors() map[MethodMetadata]CollectorConstructor {
	copied, err := copystructure.Copy(collectorRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[MethodMetadata]CollectorConstructor)
}
