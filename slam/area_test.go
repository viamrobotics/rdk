package slam

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/edaniels/test"
)

func TestNewSquareArea(t *testing.T) {
	_, err := NewSquareArea(99, 1)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "divisible")

	sa, err := NewSquareArea(100, 10)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, sa.Dim(), test.ShouldEqual, 1000)
	test.That(t, sa.QuadrantLength(), test.ShouldEqual, 500)
	sizeMeters, scale := sa.Size()
	test.That(t, sizeMeters, test.ShouldEqual, 100)
	test.That(t, scale, test.ShouldEqual, 10)
}

func TestSquareAreaWriteToFile(t *testing.T) {
	sa, err := NewSquareArea(100, 10)
	test.That(t, err, test.ShouldBeNil)
	sa.Mutate(func(area MutableArea) {
		area.Set(1, 2, 5)
		area.Set(291, 12, -1)
		area.Set(7, 6, 1)
		area.Set(1, 1, 0)
	})

	temp, err := ioutil.TempFile("", "*.las")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	err = sa.WriteToFile(temp.Name())
	test.That(t, err, test.ShouldBeNil)

	nextArea, err := NewSquareAreaFromFile(temp.Name(), 100, 10)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextArea, test.ShouldResemble, sa)
}

func TestSquareArea(t *testing.T) {
	sa, err := NewSquareArea(100, 10)
	test.That(t, err, test.ShouldBeNil)
	sa.Mutate(func(area MutableArea) {
		area.Set(1, 2, 5)
		area.Set(291, 12, -1)
		area.Set(7, 6, 1)
		area.Set(1, 1, 0)
		area.Set(-500, -500, 0)
		area.Set(0, 0, 1)
		area.Set(499, 499, 2)
		test.That(t, func() { area.Set(-501, 0, 0) }, test.ShouldPanic)
		test.That(t, func() { area.Set(0, -501, 0) }, test.ShouldPanic)
		test.That(t, func() { area.Set(500, 0, 0) }, test.ShouldPanic)
		test.That(t, func() { area.Set(0, 500, 0) }, test.ShouldPanic)
	})

	sa.Mutate(func(area MutableArea) {
		test.That(t, area.At(1, 2), test.ShouldEqual, 5)
		test.That(t, area.At(291, 12), test.ShouldEqual, -1)
		test.That(t, area.At(7, 6), test.ShouldEqual, 1)
		test.That(t, area.At(1, 1), test.ShouldEqual, 0)
		test.That(t, area.At(1, 0), test.ShouldEqual, 0)
	})

	sa.Mutate(func(area MutableArea) {
		called := 0
		area.Iterate(func(x, y, v int) bool {
			called++
			return false
		})
		test.That(t, called, test.ShouldEqual, 1)

		expected := map[string]struct{}{
			"1,2,5":       {},
			"291,12,-1":   {},
			"7,6,1":       {},
			"1,1,0":       {},
			"-500,-500,0": {},
			"0,0,1":       {},
			"499,499,2":   {},
		}
		area.Iterate(func(x, y, v int) bool {
			called++
			delete(expected, fmt.Sprintf("%d,%d,%d", x, y, v))
			return true
		})
		test.That(t, called, test.ShouldEqual, 8)
		test.That(t, expected, test.ShouldBeEmpty)
	})

	newSA := sa.BlankCopy()

	sizeMeters, scale := sa.Size()
	areaDim := sa.Dim()
	quadLen := sa.QuadrantLength()

	newSizeMeters, newScale := newSA.Size()
	newAreaDim := newSA.Dim()
	newQuadLen := newSA.QuadrantLength()

	test.That(t, sizeMeters, test.ShouldEqual, newSizeMeters)
	test.That(t, scale, test.ShouldEqual, newScale)
	test.That(t, areaDim, test.ShouldEqual, newAreaDim)
	test.That(t, quadLen, test.ShouldResemble, newQuadLen)
	test.That(t, newSA.cloud.Size(), test.ShouldEqual, 0)
}
