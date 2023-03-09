package data

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/resource"
)

var dummyCollectorConstructor = func(i interface{}, params CollectorParams) (Collector, error) {
	return &collector{}, nil
}

func TestRegister(t *testing.T) {
	defer func() {
		for k := range collectorRegistry {
			delete(collectorRegistry, k)
		}
	}()
	md := MethodMetadata{
		Subtype:    resource.NewDefaultSubtype("type", resource.ResourceTypeComponent),
		MethodName: "method",
	}
	dummyCollectorConstructor = func(i interface{}, params CollectorParams) (Collector, error) {
		return &collector{}, nil
	}

	// Return registered collector if one exists.
	RegisterCollector(md, dummyCollectorConstructor)
	ret := *CollectorLookup(md)
	test.That(t, ret, test.ShouldEqual, dummyCollectorConstructor)

	// Return nothing if exact match has not been registered.
	wrongType := MethodMetadata{
		Subtype:    resource.NewDefaultSubtype("wrongType", resource.ResourceTypeComponent),
		MethodName: "method",
	}
	wrongMethod := MethodMetadata{
		Subtype:    resource.NewDefaultSubtype("type", resource.ResourceTypeComponent),
		MethodName: "WrongMethod",
	}
	test.That(t, CollectorLookup(wrongType), test.ShouldBeNil)
	test.That(t, CollectorLookup(wrongMethod), test.ShouldBeNil)

	// Panic if try to register same thing twice.
	test.That(t, func() { RegisterCollector(md, dummyCollectorConstructor) }, test.ShouldPanic)
}
