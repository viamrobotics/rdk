package pointcloud

import (
	"bytes"
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
		test.That(b, err, test.ShouldBeNil)
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

func TestPCD(t *testing.T) {
	cloud := New()
	test.That(t, cloud.Set(NewColoredPoint(-1, -2, 5, color.NRGBA{255, 1, 2, 255}).SetValue(5)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewColoredPoint(582, 12, 0, color.NRGBA{255, 1, 2, 255}).SetValue(-1)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewColoredPoint(7, 6, 1, color.NRGBA{255, 1, 2, 255}).SetValue(1)), test.ShouldBeNil)
	/*
		The expected string is below, cannot do direct comparison because maps print out in random order.
		"VERSION .7\n" +
		"FIELDS x y z rgb\n" +
		"SIZE 4 4 4 4\n" +
		"TYPE F F F I\n" +
		"COUNT 1 1 1 1\n" +
		"WIDTH 3\n" +
		"HEIGHT 1\n" +
		"VIEWPOINT 0 0 0 1 0 0 0\n" +
		"POINTS 3\n" +
		"DATA ascii\n" +
		"-1.000000 2.000000 -5.000000 16711938\n" +
		"582.000000 -12.000000 -0.000000 16711938\n" +
		"7.000000 -6.000000 -1.000000 16711938\n"
	*/

	// write to .pcd
	var buf bytes.Buffer
	err := cloud.ToPCD(&buf)
	test.That(t, err, test.ShouldBeNil)
	gotPCD := buf.String()
	test.That(t, gotPCD, test.ShouldContainSubstring, "WIDTH 3")
	test.That(t, gotPCD, test.ShouldContainSubstring, "HEIGHT 1")
	test.That(t, gotPCD, test.ShouldContainSubstring, "POINTS 3")
	test.That(t, gotPCD, test.ShouldContainSubstring, "-1.000000 2.000000 -5.000000 16711938\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "582.000000 -12.000000 -0.000000 16711938\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "7.000000 -6.000000 -1.000000 16711938\n")
}

func TestRoundTripFileWithColorFloat(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cloud := New()
	test.That(t, cloud.Set(NewColoredPoint(-1, -2, 5, color.NRGBA{255, 1, 2, 255}).SetValue(5)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewColoredPoint(582, 12, 0, color.NRGBA{255, 1, 2, 255}).SetValue(-1)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewColoredPoint(7, 6, 1, color.NRGBA{255, 1, 2, 255}).SetValue(1)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewColoredPoint(1, 2, 9, color.NRGBA{255, 1, 2, 255}).SetValue(0)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewColoredPoint(1, 2, 9, color.NRGBA{255, 1, 2, 255}).SetValue(0)), test.ShouldBeNil)

	byts := make([]byte, 8)
	v := 1.4
	bits := math.Float64bits(v)
	binary.LittleEndian.PutUint64(byts, bits)
	outBits := binary.LittleEndian.Uint64(byts)
	outV := math.Float64frombits(outBits)
	test.That(t, outV, test.ShouldEqual, v)

	// write to .las
	temp, err := ioutil.TempFile("", "*.las")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	err = cloud.WriteToFile(temp.Name())
	test.That(t, err, test.ShouldBeNil)

	nextCloud, err := NewFromFile(temp.Name(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextCloud, test.ShouldResemble, cloud)
}
