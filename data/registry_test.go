package data

import (
	"os"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/resource"
)

var dummyCollectorConstructor = func(conn rpc.ClientConn, params map[string]string, interval time.Duration,
	target *os.File, logger golog.Logger) Collector {
	return Collector{}
}

func TestRegister(t *testing.T) {
	md := MethodMetadata{
		Subtype:    resource.SubtypeName("type"),
		MethodName: "method",
	}
	dummyCollectorConstructor = func(conn rpc.ClientConn, params map[string]string, interval time.Duration,
		target *os.File, logger golog.Logger) Collector {
		return Collector{}
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
