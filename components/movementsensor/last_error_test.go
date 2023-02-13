package movementsensor_test

import (
	"testing"

	"github.com/pkg/errors"
	ms "go.viam.com/rdk/components/movementsensor"
	"go.viam.com/test"
)

func TestNoErrors(t *testing.T) {
	le := ms.LastError{}
	test.That(t, le.Get(), test.ShouldBeNil)
}

func TestOneError(t *testing.T) {
	le := ms.LastError{}

	le.Set(errors.New("it's a test error"))
	test.That(t, le.Get(), test.ShouldNotBeNil)
	// We got the error, so it shouldn't be in here any more.
	test.That(t, le.Get(), test.ShouldBeNil)
}

func TestTwoErrors(t *testing.T) {
	le := ms.LastError{}

	le.Set(errors.New("first"))
	le.Set(errors.New("second"))

	err := le.Get()
	test.That(t, err.Error(), test.ShouldEqual, "second")
}
