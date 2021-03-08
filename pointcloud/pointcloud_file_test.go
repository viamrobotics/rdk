package pointcloud

import (
	"encoding/binary"
	"image/color"
	"io/ioutil"
	"math"
	"os"
	"testing"

	"github.com/edaniels/test"
)

func TestNewFromFile(t *testing.T) {
	cloud, err := NewFromFile("data/test.las")
	test.That(t, err, test.ShouldBeNil)
	numPoints := cloud.Size()
	test.That(t, numPoints, test.ShouldEqual, 8413)

	temp, err := ioutil.TempFile("", "*.las")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	err = cloud.WriteToFile(temp.Name())
	test.That(t, err, test.ShouldBeNil)

	nextCloud, err := NewFromFile(temp.Name())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextCloud, test.ShouldResemble, cloud)
}

func TestRoundTripFileWithColorFloat(t *testing.T) {
	cloud := New()
	test.That(t, cloud.Set(WithPointValue(NewColoredPoint(-1, -2, 5, &color.RGBA{255, 1, 2, 0}), 5)), test.ShouldBeNil)
	test.That(t, cloud.Set(WithPointValue(NewColoredPoint(582, 12, 0, &color.RGBA{255, 1, 2, 0}), -1)), test.ShouldBeNil)
	test.That(t, cloud.Set(WithPointValue(NewColoredPoint(7, 6, 1, &color.RGBA{255, 1, 2, 0}), 1)), test.ShouldBeNil)
	test.That(t, cloud.Set(WithPointValue(NewColoredPoint(1, 2, 9, &color.RGBA{255, 1, 2, 0}), 0)), test.ShouldBeNil)
	test.That(t, cloud.Set(WithPointValue(NewColoredPoint(1, 2, 9, &color.RGBA{255, 1, 2, 0}), 0)), test.ShouldBeNil)

	bytes := make([]byte, 8)
	v := 1.4
	bits := math.Float64bits(v)
	binary.LittleEndian.PutUint64(bytes, bits)
	outBits := binary.LittleEndian.Uint64(bytes)
	outV := math.Float64frombits(outBits)
	test.That(t, outV, test.ShouldEqual, v)

	temp, err := ioutil.TempFile("", "*.las")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	err = cloud.WriteToFile(temp.Name())
	test.That(t, err, test.ShouldBeNil)

	nextCloud, err := NewFromFile(temp.Name())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextCloud, test.ShouldResemble, cloud)
}
