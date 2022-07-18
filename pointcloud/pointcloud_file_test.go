package pointcloud

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"io/ioutil"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
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

	err = WriteToLASFile(cloud, temp.Name())
	test.That(t, err, test.ShouldBeNil)

	nextCloud, err := NewFromFile(temp.Name(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextCloud, test.ShouldResemble, cloud)
}

func TestPCD(t *testing.T) {
	cloud := New()
	test.That(t, cloud.Set(NewVector(-1, -2, 5), NewColoredData(color.NRGBA{255, 1, 2, 255}).SetValue(5)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewVector(582, 12, 0), NewColoredData(color.NRGBA{255, 1, 2, 255}).SetValue(-1)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewVector(7, 6, 1), NewColoredData(color.NRGBA{255, 1, 2, 255}).SetValue(1)), test.ShouldBeNil)
	test.That(t, cloud.Size(), test.ShouldEqual, 3)
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
		"-0.001000 0.002000 0.005000 16711938\n" +
		"0.582000 0.012000 0.000000 16711938\n" +
		"0.007000 0.006000 0.001000 16711938\n"
	*/

	testASCIIRoundTrip(t, cloud)
	testBinaryRoundTrip(t, cloud)
}

func TestPCDNoColor(t *testing.T) {
	cloud := New()
	test.That(t, cloud.Set(NewVector(-1, -2, 5), NewBasicData()), test.ShouldBeNil)
	test.That(t, cloud.Set(NewVector(582, 12, 0), NewBasicData()), test.ShouldBeNil)
	test.That(t, cloud.Set(NewVector(7, 6, 1), NewBasicData()), test.ShouldBeNil)
	test.That(t, cloud.Size(), test.ShouldEqual, 3)

	testNoColorASCIIRoundTrip(t, cloud)
	testNoColorBinaryRoundTrip(t, cloud)
	testLargeBinaryNoError(t)
}

func testNoColorASCIIRoundTrip(t *testing.T, cloud PointCloud) {
	t.Helper()
	// write to .pcd
	var buf bytes.Buffer
	err := ToPCD(cloud, &buf, PCDAscii)
	test.That(t, err, test.ShouldBeNil)
	gotPCD := buf.String()
	test.That(t, gotPCD, test.ShouldContainSubstring, "WIDTH 3\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "HEIGHT 1\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "POINTS 3\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "DATA ascii\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "FIELDS x y z\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "-0.001000 -0.002000 0.005000\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "0.582000 0.012000 0.000000\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "0.007000 0.006000 0.001000\n")

	cloud2, err := ReadPCD(strings.NewReader(gotPCD))
	test.That(t, err, test.ShouldBeNil)
	testPCDOutput(t, cloud2)

	_, err = ReadPCD(strings.NewReader(gotPCD[1:]))
	test.That(t, err, test.ShouldNotBeNil)

	_, err = ReadPCD(strings.NewReader("VERSION .8\n" + gotPCD[11:]))
	test.That(t, err, test.ShouldNotBeNil)
}

func testNoColorBinaryRoundTrip(t *testing.T, cloud PointCloud) {
	t.Helper()
	// write to .pcd
	var buf bytes.Buffer
	err := ToPCD(cloud, &buf, PCDBinary)
	test.That(t, err, test.ShouldBeNil)
	gotPCD := buf.String()
	test.That(t, gotPCD, test.ShouldContainSubstring, "WIDTH 3\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "HEIGHT 1\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "POINTS 3\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "FIELDS x y z\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "DATA binary\n")

	cloud2, err := ReadPCD(strings.NewReader(gotPCD))
	test.That(t, err, test.ShouldBeNil)
	testPCDOutput(t, cloud2)

	_, err = ReadPCD(strings.NewReader(gotPCD[1:]))
	test.That(t, err.Error(), test.ShouldContainSubstring, "line is supposed to start with")

	_, err = ReadPCD(strings.NewReader("VERSION .8\n" + gotPCD[11:]))
	test.That(t, err.Error(), test.ShouldContainSubstring, "unsupported pcd version")
}

func testPCDOutput(t *testing.T, cloud2 PointCloud) {
	t.Helper()
	test.That(t, cloud2.Size(), test.ShouldEqual, 3)
	test.That(t, CloudContains(cloud2, 0, 0, 0), test.ShouldBeFalse)
	test.That(t, CloudContains(cloud2, -1, -2, 5), test.ShouldBeTrue)
}

func testASCIIRoundTrip(t *testing.T, cloud PointCloud) {
	t.Helper()
	// write to .pcd
	var buf bytes.Buffer
	err := ToPCD(cloud, &buf, PCDAscii)
	test.That(t, err, test.ShouldBeNil)
	gotPCD := buf.String()
	test.That(t, gotPCD, test.ShouldContainSubstring, "WIDTH 3\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "HEIGHT 1\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "POINTS 3\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "DATA ascii\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "-0.001000 -0.002000 0.005000 16711938\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "0.582000 0.012000 0.000000 16711938\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "0.007000 0.006000 0.001000 16711938\n")

	cloud2, err := ReadPCD(strings.NewReader(gotPCD))
	test.That(t, err, test.ShouldBeNil)
	testPCDOutput(t, cloud2)

	_, err = ReadPCD(strings.NewReader(gotPCD[1:]))
	test.That(t, err, test.ShouldNotBeNil)

	_, err = ReadPCD(strings.NewReader("VERSION .8\n" + gotPCD[11:]))
	test.That(t, err, test.ShouldNotBeNil)
}

func testBinaryRoundTrip(t *testing.T, cloud PointCloud) {
	t.Helper()
	// write to .pcd
	var buf bytes.Buffer
	err := ToPCD(cloud, &buf, PCDBinary)
	test.That(t, err, test.ShouldBeNil)
	gotPCD := buf.String()
	test.That(t, gotPCD, test.ShouldContainSubstring, "WIDTH 3\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "HEIGHT 1\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "POINTS 3\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "DATA binary\n")

	cloud2, err := ReadPCD(strings.NewReader(gotPCD))
	test.That(t, err, test.ShouldBeNil)
	testPCDOutput(t, cloud2)

	_, err = ReadPCD(strings.NewReader(gotPCD[1:]))
	test.That(t, err, test.ShouldNotBeNil)

	_, err = ReadPCD(strings.NewReader("VERSION .8\n" + gotPCD[11:]))
	test.That(t, err, test.ShouldNotBeNil)
}

func testLargeBinaryNoError(t *testing.T) {
	// This tests whether large pointclouds that exceed the usual buffered page size for a file error on reads
	t.Helper()
	var buf bytes.Buffer
	largeCloud := newBigPC()
	err := ToPCD(largeCloud, &buf, PCDBinary)
	test.That(t, err, test.ShouldBeNil)

	readPointCloud, err := ReadPCD(strings.NewReader(buf.String()))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readPointCloud.Size(), test.ShouldEqual, largeCloud.Size())
}

func TestRoundTripFileWithColorFloat(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cloud := New()
	test.That(t, cloud.Set(NewVector(-1, -2, 5), NewColoredData(color.NRGBA{255, 1, 2, 255}).SetValue(5)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewVector(582, 12, 0), NewColoredData(color.NRGBA{255, 1, 2, 255}).SetValue(-1)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewVector(7, 6, 1), NewColoredData(color.NRGBA{255, 1, 2, 255}).SetValue(1)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewVector(1, 2, 9), NewColoredData(color.NRGBA{255, 1, 2, 255}).SetValue(0)), test.ShouldBeNil)
	test.That(t, cloud.Set(NewVector(1, 2, 9), NewColoredData(color.NRGBA{255, 1, 2, 255}).SetValue(0)), test.ShouldBeNil)

	floatBytes := make([]byte, 8)
	v := 1.4
	bits := math.Float64bits(v)
	binary.LittleEndian.PutUint64(floatBytes, bits)
	outBits := binary.LittleEndian.Uint64(floatBytes)
	outV := math.Float64frombits(outBits)
	test.That(t, outV, test.ShouldEqual, v)

	// write to .las
	temp, err := ioutil.TempFile("", "*.las")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	err = WriteToLASFile(cloud, temp.Name())
	test.That(t, err, test.ShouldBeNil)

	nextCloud, err := NewFromFile(temp.Name(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextCloud, test.ShouldResemble, cloud)
}

func TestPCDColor(t *testing.T) {
	c := color.NRGBA{5, 31, 123, 255}
	p := NewColoredData(c)
	x := _colorToPCDInt(p)
	c2 := _pcdIntToColor(x)
	test.That(t, c, test.ShouldResemble, c2)
}

func newBigPC() PointCloud {
	cloud := New()
	for x := 10.0; x <= 50; x++ {
		for y := 10.0; y <= 50; y++ {
			for z := 10.0; z <= 50; z++ {
				if err := cloud.Set(NewVector(x, y, z), NewColoredData(color.NRGBA{255, 1, 2, 255}).SetValue(5)); err != nil {
					panic(err)
				}
			}
		}
	}
	return cloud
}

func BenchmarkPCDASCIIWrite(b *testing.B) {
	cloud := newBigPC()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		err := ToPCD(cloud, &buf, PCDAscii)
		test.That(b, err, test.ShouldBeNil)
	}
}

func BenchmarkPCDASCIIRead(b *testing.B) {
	cloud := newBigPC()
	var buf bytes.Buffer
	err := ToPCD(cloud, &buf, PCDAscii)
	test.That(b, err, test.ShouldBeNil)

	gotPCD := buf.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ReadPCD(strings.NewReader(gotPCD))
		test.That(b, err, test.ShouldBeNil)
	}
}

func BenchmarkPCDBinaryWrite(b *testing.B) {
	cloud := newBigPC()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		err := ToPCD(cloud, &buf, PCDBinary)
		test.That(b, err, test.ShouldBeNil)
	}
}

func BenchmarkPCDBinaryRead(b *testing.B) {
	cloud := newBigPC()
	var buf bytes.Buffer
	err := ToPCD(cloud, &buf, PCDBinary)
	test.That(b, err, test.ShouldBeNil)

	gotPCD := buf.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ReadPCD(strings.NewReader(gotPCD))
		test.That(b, err, test.ShouldBeNil)
	}
}
