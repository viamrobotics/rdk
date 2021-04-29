package utils

import (
	"errors"
	"testing"

	"github.com/edaniels/test"
	"go.uber.org/multierr"
)

func TestFilterOutError(t *testing.T) {
	test.That(t, FilterOutError(nil, nil), test.ShouldBeNil)
	err1 := errors.New("error1")
	test.That(t, FilterOutError(err1, nil), test.ShouldEqual, err1)
	test.That(t, FilterOutError(err1, err1), test.ShouldBeNil)
	err2 := errors.New("error2")
	test.That(t, FilterOutError(err1, err2), test.ShouldEqual, err1)
	err3 := errors.New("error") // substring
	test.That(t, FilterOutError(err1, err3), test.ShouldBeNil)
	err4 := errors.New("error4")
	errM := multierr.Combine(err1, err2, err4)
	test.That(t, FilterOutError(errM, err2), test.ShouldResemble, multierr.Combine(err1, err4))
	test.That(t, FilterOutError(errM, err3), test.ShouldBeNil)
}

func TestNewConfigValidationError(t *testing.T) {
	err := NewConfigValidationError("thing", errors.New("another one"))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "thing")
	test.That(t, err.Error(), test.ShouldContainSubstring, "another one")

	err = NewConfigValidationFieldRequiredError("thing", "another")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "thing")
	test.That(t, err.Error(), test.ShouldContainSubstring, `"another" is required`)
}
