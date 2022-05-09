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
	md := MethodMetadata{
		Subtype:    resource.SubtypeName("type"),
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
		Subtype:    resource.SubtypeName("wrongType"),
		MethodName: "method",
	}
	wrongMethod := MethodMetadata{
		Subtype:    resource.SubtypeName("type"),
		MethodName: "WrongMethod",
	}
	test.That(t, CollectorLookup(wrongType), test.ShouldBeNil)
	test.That(t, CollectorLookup(wrongMethod), test.ShouldBeNil)

	// Panic if try to register same thing twice.
	test.That(t, func() { RegisterCollector(md, dummyCollectorConstructor) }, test.ShouldPanic)
}
