package data

import (
	"context"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

type MethodMetadata struct {
	// TODO: component? or subtype?
	ComponentName string
	MethodName    string
}

type CollectorSchema struct {
	ServiceClient interface{} // TODO: do we need this? Or is just the method sufficient?
	Method        func(ctx context.Context, in *any.Any, opts ...grpc.CallOption) (any.Any, error)
	// TODO: include input/output type literals so those any.Any can be casted.
	// TODO: minimum capture interval? Though unsure if that exists in code yet.
	Params []string
}

var (
	collectorRegistry = map[MethodMetadata]CollectorSchema{}
)

// RegisterCollector registers a Collector to its corresponding component subtype.
func RegisterCollector(method MethodMetadata, schema CollectorSchema) {
	_, old := collectorRegistry[method]
	if old {
		panic(errors.Errorf("trying to register two of the same method on the same component: "+
			"component %s, method %s", method.ComponentName, method.MethodName))
	}
	if schema.ServiceClient == nil || schema.Method == nil {
		panic(errors.Errorf("cannot register a data collector with a nil client or method"))
	}

	collectorRegistry[method] = schema
}

// CollectorLookup looks up a Collector by the given subtype. nil is returned if
// there is None.
func CollectorLookup(method MethodMetadata) *CollectorSchema {
	if registration, ok := RegisteredCollectors()[method]; ok {
		return &registration
	}
	return nil
}

// RegisteredCollectors returns a copy of the registered CollectorSchema.
func RegisteredCollectors() map[MethodMetadata]CollectorSchema {
	copied, err := copystructure.Copy(collectorRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[MethodMetadata]CollectorSchema)
}
