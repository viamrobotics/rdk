package ik

import (
	"testing"

	"go.viam.com/test"
)

func TestThirds(t *testing.T) {
	test.That(t, bottomThird(0, 4), test.ShouldBeTrue)
	test.That(t, bottomThird(1, 4), test.ShouldBeTrue)
	test.That(t, bottomThird(2, 4), test.ShouldBeFalse)

	test.That(t, middleThird(2, 4), test.ShouldBeTrue)
	test.That(t, middleThird(3, 4), test.ShouldBeFalse)

	test.That(t, bottomThird(2, 10), test.ShouldBeTrue)
	test.That(t, bottomThird(3, 10), test.ShouldBeTrue)
	test.That(t, bottomThird(4, 10), test.ShouldBeFalse)

	test.That(t, middleThird(5, 10), test.ShouldBeTrue)
	test.That(t, middleThird(6, 10), test.ShouldBeTrue)
	test.That(t, middleThird(7, 10), test.ShouldBeFalse)

	test.That(t, bottomThird(0, 2), test.ShouldBeFalse)
	test.That(t, middleThird(0, 2), test.ShouldBeTrue)
	test.That(t, middleThird(1, 2), test.ShouldBeFalse)
}
