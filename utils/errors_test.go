package utils

import (
	"testing"

	"go.viam.com/test"
)

func TestDependencyTypeError(t *testing.T) {
	type tc struct {
		name     string
		expected interface{}
		actual   interface{}
		errStr   string
	}
	t1 := tc{"one", "exp1", "actual1", `dependency "one" should be an implementation of *string but it was a string`}
	err := DependencyTypeError[string](t1.name, t1.actual)
	test.That(t, err.Error(), test.ShouldContainSubstring, t1.errStr)

	t1 = tc{"two", 1, "actual2", `dependency "two" should be an implementation of *int but it was a string`}
	err = DependencyTypeError[int](t1.name, t1.actual)
	test.That(t, err.Error(), test.ShouldContainSubstring, t1.errStr)

	// the WRONG way to use this
	t1 = tc{"three", (someIfc)(nil), 4, `dependency "three" should be an implementation of utils.someIfc but it was a int`}
	err = DependencyTypeError[someIfc](t1.name, t1.actual)
	test.That(t, err.Error(), test.ShouldContainSubstring, t1.errStr)

	// the right way to use this
	t1 = tc{"four", (*someIfc)(nil), 5, `dependency "four" should be an implementation of utils.someIfc but it was a int`}
	err = DependencyTypeError[someIfc](t1.name, t1.actual)
	test.That(t, err.Error(), test.ShouldContainSubstring, t1.errStr)

	t1 = tc{"five", (*someStruct)(nil), 6, `dependency "five" should be an implementation of *utils.someStruct but it was a int`}
	err = DependencyTypeError[someStruct](t1.name, t1.actual)
	test.That(t, err.Error(), test.ShouldContainSubstring, t1.errStr)

	t1 = tc{"six", someStruct{}, 7, `dependency "six" should be an implementation of *utils.someStruct but it was a int`}
	err = DependencyTypeError[someStruct](t1.name, t1.actual)
	test.That(t, err.Error(), test.ShouldContainSubstring, t1.errStr)
}

func TestNewUnexpectedTypeError(t *testing.T) {
	for _, tc := range []struct {
		name     string
		expected interface{}
		actual   interface{}
		errStr   string
	}{
		{"one", "exp1", "actual1", `expected string but got string`},
		{"two", 1, "actual2", `expected int but got string`},
		{"three", nil, "actual3", `expected <unknown (nil interface)> but got string`},

		// the WRONG way to use this
		{"four", (someIfc)(nil), 4, `expected <unknown (nil interface)> but got int`},

		// the right way to use this
		{"five", (*someIfc)(nil), 5, `expected utils.someIfc but got int`},

		{"six", (*someStruct)(nil), 6, `expected *utils.someStruct but got int`},
		{"seven", someStruct{}, 7, `expected utils.someStruct but got int`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := NewUnexpectedTypeError(tc.expected, tc.actual)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.errStr)
		})
	}
}

func TestNewUnimplementedInterfaceError(t *testing.T) {
	for _, tc := range []struct {
		name     string
		expected interface{}
		actual   interface{}
		errStr   string
	}{
		{"one", "exp1", "actual1", `expected implementation of string but got string`},
		{"two", 1, "actual2", `expected implementation of int but got string`},
		{"three", nil, "actual3", `expected implementation of <unknown (nil interface)> but got string`},

		// the WRONG way to use this
		{"four", (someIfc)(nil), 4, `expected implementation of <unknown (nil interface)> but got int`},

		// the right way to use this
		{"five", (*someIfc)(nil), 5, `expected implementation of utils.someIfc but got int`},

		{"six", (*someStruct)(nil), 6, `expected implementation of *utils.someStruct but got int`},
		{"seven", someStruct{}, 7, `expected implementation of utils.someStruct but got int`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := NewUnimplementedInterfaceError(tc.expected, tc.actual)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.errStr)
		})
	}
}

type (
	someStruct struct{}
	someIfc    interface{}
)
