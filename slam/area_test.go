package slam

import (
	"image"
	"io/ioutil"
	"os"
	"testing"

	"github.com/edaniels/test"
)

func TestNewSquareArea(t *testing.T) {
	sa := NewSquareArea(100, 10)

	test.That(t, sa.Center(), test.ShouldResemble, image.Point{500, 500})
	sizeMeters, scale := sa.Size()
	test.That(t, sizeMeters, test.ShouldEqual, 100)
	test.That(t, scale, test.ShouldEqual, 10)
}

func TestSquareAreaWriteToFile(t *testing.T) {
	sa := NewSquareArea(100, 10)
	sa.Mutate(func(area MutableArea) {
		area.Set(1, 2, 5)
		area.Set(582, 12, -1)
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
