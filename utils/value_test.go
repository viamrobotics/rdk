package utils

import (
	"math/rand"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
)

func TestAssertType(t *testing.T) {
	one := 1
	_, err := AssertType[string](one)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, NewUnexpectedTypeError[string](one))

	_, err = AssertType[myAssertIfc](one)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, NewUnexpectedTypeError[myAssertIfc](one))

	asserted, err := AssertType[myAssertIfc](myAssertInt(one))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, asserted.method1(), test.ShouldBeError, errors.New("cool 8)"))
}

type myAssertIfc interface {
	method1() error
}

type myAssertInt int

func (m myAssertInt) method1() error {
	return errors.New("cool 8)")
}

func TestFilterMap(t *testing.T) {
	ret := FilterMap(map[string]int{"x": 1, "y": 2}, func(_ string, val int) bool { return val > 1 })
	test.That(t, ret, test.ShouldResemble, map[string]int{"y": 2})
}

func TestTesting(t *testing.T) {
	test.That(t, Testing(), test.ShouldBeTrue)
}

func TestSafeRand(t *testing.T) {
	instance := SafeTestingRand()
	source := rand.New(rand.NewSource(0))
	test.That(t, instance.Float64(), test.ShouldEqual, source.Float64())
}
