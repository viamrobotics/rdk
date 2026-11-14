package web

import (
	"testing"

	"go.uber.org/goleak"
	testutilsext "go.viam.com/utils/testutils/ext"
)

// TestMain is used to control the execution of all tests run within this package (including _test packages).
func TestMain(m *testing.M) {
	// A cancelled LookupIP leaves resolver goroutines running until the DNS
	// query finishes. mDNS and WebRTC dials in this package start those
	// lookups. See golang/go issues 8602, 20703, and 22724.
	testutilsext.VerifyTestMain(m,
		testutilsext.WithLeakOpt(goleak.IgnoreAnyFunction("net.(*Resolver).lookupIPAddr.func1")),
		testutilsext.WithLeakOpt(goleak.IgnoreAnyFunction("net.(*Resolver).lookupIPAddr.func2")),
		testutilsext.WithLeakOpt(goleak.IgnoreAnyFunction("net.(*Resolver).lookupIP")),
		testutilsext.WithLeakOpt(goleak.IgnoreAnyFunction("net.(*Resolver).exchange")),
	)
}
