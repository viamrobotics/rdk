package movementsensor

import (
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
)

func TestNoErrors(t *testing.T) {
	le := lastError{}
	test.That(t, le.Get(), test.ShouldBeNil)
}

func TestOneError(t *testing.T) {
	le := lastError{}

	le.Set(errors.New("it's a test error"))
	test.That(t, le.Get(), test.ShouldNotBeNil)
	// We got the error, so it shouldn't be in here any more.
	test.That(t, le.Get(), test.ShouldBeNil)
}

func TestTwoErrors(t *testing.T) {
	le := lastError{}

	le.Set(errors.New("first"))
	le.Set(errors.New("second"))

	err := le.Get()
	test.That(t, err.Error(), test.ShouldEqual, "second")
}
