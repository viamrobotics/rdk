package utils

import (
	"math/rand"
	"os/exec"
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
	cmd := exec.Command("go", "run", "./test_detector")
	cmd.Start()
	cmd.Wait()
	test.That(t, cmd.ProcessState.ExitCode(), test.ShouldEqual, 0)
}

func TestSafeRand(t *testing.T) {
	instance := SafeTestingRand()
	source := rand.New(rand.NewSource(0))
	test.That(t, instance.Float64(), test.ShouldEqual, source.Float64())
}

func TestFindInSlice(t *testing.T) {
	filtered := FindInSlice([]int{1, 2, 3}, func(x int) bool { return x > 2 })
	test.That(t, filtered, test.ShouldNotBeNil)
	test.That(t, *filtered, test.ShouldEqual, 3)
}

func TestMapOver(t *testing.T) {
	mapped, _ := MapOver([]int{1, 2}, func(x int) (int, error) { return x + 1, nil })
	test.That(t, mapped, test.ShouldResemble, []int{2, 3})
}

func TestFilterSlice(t *testing.T) {
	filtered := FilterSlice([]int{1, 2, 3, 4, 5}, func(x int) bool { return x%2 == 0 })
	test.That(t, filtered, test.ShouldResemble, []int{2, 4})
}

func TestSanitizePath(t *testing.T) {
	test.That(t, SanitizePath("../.123"), test.ShouldResemble, "..-.123")
}
