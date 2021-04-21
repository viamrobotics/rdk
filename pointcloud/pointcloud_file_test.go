package pointcloud

import (
	"encoding/binary"
	"image/color"
	"io/ioutil"
	"math"
	"os"
	"testing"

	"go.viam.com/robotcore/artifact"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
)

func BenchmarkNewFromFile(b *testing.B) {
	logger := golog.NewLogger("pointcloud_benchmark")
	for i := 0; i < b.N; i++ {
		_, err := NewFromFile(artifact.MustPath("pointcloud/test.las"), logger)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestNewFromFile(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cloud, err := NewFromFile(artifact.MustPath("pointcloud/test.las"), logger)
	test.That(t, err, test.ShouldBeNil)
	numPoints := cloud.Size()
	test.That(t, numPoints, test.ShouldEqual, 8413)

	temp, err := ioutil.TempFile("", "*.las")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	err = cloud.WriteToFile(temp.Name())
	test.That(t, err, test.ShouldBeNil)

	nextCloud, err := NewFromFile(temp.Name(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextCloud, test.ShouldResemble, cloud)
}

func TestRoundTripFileWithColorFloat(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cloud := New()
	test.That(t, cloud.Set(NewColoredPoint(-1, -2, 5, color.NRGBA{255, 1, 2, 255}).SetValue(5)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewColoredPoint(582, 12, 0, color.NRGBA{255, 1, 2, 255}).SetValue(-1)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewColoredPoint(7, 6, 1, color.NRGBA{255, 1, 2, 255}).SetValue(1)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewColoredPoint(1, 2, 9, color.NRGBA{255, 1, 2, 255}).SetValue(0)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewColoredPoint(1, 2, 9, color.NRGBA{255, 1, 2, 255}).SetValue(0)), test.ShouldBeNil)

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

	nextCloud, err := NewFromFile(temp.Name(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextCloud, test.ShouldResemble, cloud)
}
