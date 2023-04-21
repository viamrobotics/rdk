package utils

import (
	"testing"

	"go.viam.com/test"
)

func TestNewUnexpectedTypeError(t *testing.T) {
	err := NewUnexpectedTypeError[string]("actual1")
	test.That(t, err.Error(), test.ShouldContainSubstring, `expected string but got string`)

	err = NewUnexpectedTypeError[int]("actual2")
	test.That(t, err.Error(), test.ShouldContainSubstring, `expected int but got string`)

	err = NewUnexpectedTypeError[*someStruct](3)
	test.That(t, err.Error(), test.ShouldContainSubstring, `expected *utils.someStruct but got int`)

	err = NewUnexpectedTypeError[someStruct](4)
	test.That(t, err.Error(), test.ShouldContainSubstring, `expected utils.someStruct but got int`)

	err = NewUnexpectedTypeError[someIfc](5)
	test.That(t, err.Error(), test.ShouldContainSubstring, `expected utils.someIfc but got int`)

	err = NewUnexpectedTypeError[*someIfc](6)
	test.That(t, err.Error(), test.ShouldContainSubstring, `expected *utils.someIfc but got int`)
}

type (
	someStruct struct{}
	someIfc    interface{}
)
