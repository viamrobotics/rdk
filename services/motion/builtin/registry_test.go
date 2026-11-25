package builtin

import (
	"testing"

	"go.viam.com/test"
)

func TestRegistryHolders(t *testing.T) {
	r := newRegistry()
	test.That(t, r.claim("arm", holder{kind: holderServo, servo: &servoEntry{}}), test.ShouldBeNil)
	err := r.claim("arm", holder{kind: holderStream, stream: &stream{}})
	test.That(t, err.Error(), test.ShouldContainSubstring, `arm "arm" is held by a servo session`)
	r.release("arm", holderServo)
	test.That(t, r.claim("arm", holder{kind: holderStream, stream: &stream{}}), test.ShouldBeNil)
	// A stream holding the arm keeps today's wording, which stream_test.go asserts.
	err = r.claim("arm", holder{kind: holderServo, servo: &servoEntry{}})
	test.That(t, err.Error(), test.ShouldContainSubstring, `a stream is already running for arm "arm"`)
	r.release("arm", holderStream)
	// Teleop re-claiming an arm it already holds (a second teleop_start for the same component) is a no-op.
	test.That(t, r.claim("arm", holder{kind: holderTeleop}), test.ShouldBeNil)
	test.That(t, r.claim("arm", holder{kind: holderTeleop}), test.ShouldBeNil)
	err = r.claim("arm", holder{kind: holderServo, servo: &servoEntry{}})
	test.That(t, err.Error(), test.ShouldContainSubstring, `arm "arm" is held by teleop`)
	// release by the wrong kind is a no-op
	r.release("arm", holderServo)
	_, ok := r.get("arm")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, len(r.all(holderTeleop)), test.ShouldEqual, 1)
	test.That(t, len(r.all(holderStream)), test.ShouldEqual, 0)
}
