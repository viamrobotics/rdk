package slam

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

func TestNewSquareArea(t *testing.T) {
	logger := golog.NewTestLogger(t)
	_, err := NewSquareArea(99, 1, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "divisible")

	sa, err := NewSquareArea(100, 10, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, sa.Dim(), test.ShouldEqual, 1000)
	test.That(t, sa.QuadrantLength(), test.ShouldEqual, 500)
	sizeMeters, unitsPerMeter := sa.Size()
	test.That(t, sizeMeters, test.ShouldEqual, 100)
	test.That(t, unitsPerMeter, test.ShouldEqual, 10)
}

func TestSquareAreaWriteToFile(t *testing.T) {
	logger := golog.NewTestLogger(t)
	sa, err := NewSquareArea(100, 10, logger)
	test.That(t, err, test.ShouldBeNil)
	sa.Mutate(func(area MutableArea) {
		test.That(t, area.Set(1, 2, 5), test.ShouldBeNil)
		test.That(t, area.Set(291, 12, -1), test.ShouldBeNil)
		test.That(t, area.Set(7, 6, 1), test.ShouldBeNil)
		test.That(t, area.Set(1, 1, 0), test.ShouldBeNil)
	})

	temp, err := ioutil.TempFile("", "*.las")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	err = sa.WriteToFile(temp.Name(), logger)
	test.That(t, err, test.ShouldBeNil)

	nextArea, err := NewSquareAreaFromFile(temp.Name(), 100, 10, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextArea, test.ShouldResemble, sa)
}

func TestSquareArea(t *testing.T) {
	logger := golog.NewTestLogger(t)
	sa, err := NewSquareArea(100, 10, logger)
	test.That(t, err, test.ShouldBeNil)
	sa.Mutate(func(area MutableArea) {
		test.That(t, area.Set(1, 2, 5), test.ShouldBeNil)
		test.That(t, area.Set(291, 12, -1), test.ShouldBeNil)
		test.That(t, area.Set(7, 6, 1), test.ShouldBeNil)
		test.That(t, area.Set(1, 1, 0), test.ShouldBeNil)
		test.That(t, area.Set(-500, -500, 0), test.ShouldBeNil)
		test.That(t, area.Set(0, 0, 1), test.ShouldBeNil)
		test.That(t, area.Set(499, 499, 2), test.ShouldBeNil)
		err := area.Set(-501, 0, 0)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "x must be")
		err = area.Set(0, -501, 0)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "y must be")
		err = area.Set(500, 0, 0)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "x must be")
		err = area.Set(0, 500, 0)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "y must be")
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

	newSA, err := sa.BlankCopy(logger)
	test.That(t, err, test.ShouldBeNil)

	sizeMeters, unitsPerMeter := sa.Size()
	areaDim := sa.Dim()
	quadLen := sa.QuadrantLength()

	newSizeMeters, newUnitsPerMeter := newSA.Size()
	newAreaDim := newSA.Dim()
	newQuadLen := newSA.QuadrantLength()

	test.That(t, sizeMeters, test.ShouldEqual, newSizeMeters)
	test.That(t, newUnitsPerMeter, test.ShouldEqual, unitsPerMeter)
	test.That(t, areaDim, test.ShouldEqual, newAreaDim)
	test.That(t, quadLen, test.ShouldResemble, newQuadLen)
	test.That(t, newSA.cloud.Size(), test.ShouldEqual, 0)
}
