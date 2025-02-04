package grpc

import (
	"reflect"
	"testing"

	"go.viam.com/test"
)

func TestTrackerImplementations(t *testing.T) {
	tracker := reflect.TypeOf((*Tracker)(nil)).Elem()

	t.Run("*ReconfigurableClientConn should implement Tracker", func(t *testing.T) {
		test.That(t, reflect.TypeOf(&ReconfigurableClientConn{}).Implements(tracker), test.ShouldBeTrue)
	})

	t.Run("*SharedConn should implement Tracker", func(t *testing.T) {
		test.That(t, reflect.TypeOf(&SharedConn{}).Implements(tracker), test.ShouldBeTrue)
	})
}
