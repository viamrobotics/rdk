package utils

import (
	"testing"

	"go.viam.com/test"
)

func TestRefCountedValue(t *testing.T) {
	rcv := NewRefCountedValue(nil)
	test.That(t, func() { rcv.Deref() }, test.ShouldPanic)
	test.That(t, rcv.Ref(), test.ShouldBeNil)
	test.That(t, rcv.Ref(), test.ShouldBeNil)
	test.That(t, rcv.Deref(), test.ShouldBeFalse)
	test.That(t, rcv.Deref(), test.ShouldBeTrue)
	test.That(t, func() { rcv.Deref() }, test.ShouldPanic)
	test.That(t, func() { rcv.Ref() }, test.ShouldPanic)

	someIntPtr := 5
	rcv = NewRefCountedValue(&someIntPtr)
	test.That(t, func() { rcv.Deref() }, test.ShouldPanic)
	test.That(t, rcv.Ref(), test.ShouldEqual, &someIntPtr)
	test.That(t, rcv.Ref(), test.ShouldEqual, &someIntPtr)
	test.That(t, rcv.Deref(), test.ShouldBeFalse)
	test.That(t, rcv.Deref(), test.ShouldBeTrue)
	test.That(t, func() { rcv.Deref() }, test.ShouldPanic)
	test.That(t, func() { rcv.Ref() }, test.ShouldPanic)
}
