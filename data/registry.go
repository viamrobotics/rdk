package data

import (
	"context"
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils/rpc"
	"os"
	"time"
)

type Collector interface {
	Collect(ctx context.Context) error
	Close()
	GetTarget() *os.File
	SetTarget(file *os.File)
}

type CollectorConstructor struct {
	Constructor func(conn rpc.ClientConn, name string, interval time.Duration, target *os.File) interface{}
}

type MethodMetadata struct {
	// TODO: Subtype or SubType?
	Subtype    resource.Subtype
	MethodName string
}

// TODO: rethink what this should be storing. Should have everything needed so some future Data Manager Service can
// take a CollectionSchema, and:
// - Call the appropriate proto method
// - Map string params -> Proto method inputs
// - Read/deserialize those proto method outputs

/**
What do we need to call the appropriate proto method?
 - a client or a way to get the client
 - a method literal, or if you can't store those in Go structs, a method name (then reflection...)
*/
//type CollectorSchema struct {
//	// Can use this to look up client
//	ResourceSubtype resource.Subtype
//	Method          func(context context.Context) (interface{}, error)
//	// TODO: include input/output type literals so those any.Any can be casted.
//	// TODO: minimum capture interval? Though unsure if that exists in code yet.
//	// TODO: find "parent" type of pb.XSericeClient
//	RPCClient  registry.CreateRPCClient
//	InputType  any.Any
//	OutputType any.Any
//}

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
