package pointcloud

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"math"
	"os"
	"strings"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/logging"
)

func BenchmarkNewFromFile(b *testing.B) {
	logger := logging.NewTestLogger(b)
	for i := 0; i < b.N; i++ {
		_, err := NewFromFile(artifact.MustPath("pointcloud/test.las"), logger)
		test.That(b, err, test.ShouldBeNil)
	}
}

func TestNewFromFile(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cloud, err := NewFromFile(artifact.MustPath("pointcloud/test.las"), logger)
	test.That(t, err, test.ShouldBeNil)
	numPoints := cloud.Size()
	test.That(t, numPoints, test.ShouldEqual, 8413)

	temp, err := os.CreateTemp(t.TempDir(), "*.las")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	err = WriteToLASFile(cloud, temp.Name())
	test.That(t, err, test.ShouldBeNil)

	nextCloud, err := NewFromFile(temp.Name(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextCloud, test.ShouldResemble, cloud)

	cloud, err = NewFromFile(artifact.MustNewPath("pointcloud/test.pcd"), logger)
	test.That(t, err, test.ShouldBeNil)
	numPoints = cloud.Size()
	test.That(t, numPoints, test.ShouldEqual, 293363)

	tempPCD, err := os.CreateTemp(t.TempDir(), "*.pcd")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(tempPCD.Name())

	err = ToPCD(cloud, tempPCD, PCDAscii)
	test.That(t, err, test.ShouldBeNil)

	nextCloud, err = NewFromFile(tempPCD.Name(), logger)
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
	testPCDHeaders(t)
	testASCIIRoundTrip(t, cloud)
	testBinaryRoundTrip(t, cloud)
}

func testPCDHeaders(t *testing.T) {
	t.Helper()

	fakeHeader := pcdHeader{}
	var err error
	// VERSION
	err = parsePCDHeaderLine("VERSION .7", 0, &fakeHeader)
	test.That(t, err, test.ShouldBeNil)
	err = parsePCDHeaderLine("VERSION 0.7", 0, &fakeHeader)
	test.That(t, err, test.ShouldBeNil)
	err = parsePCDHeaderLine("VERSION .8", 0, &fakeHeader)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unsupported pcd version")
	// FIELDS
	err = parsePCDHeaderLine("FIELDS x y z rgb", 1, &fakeHeader)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeHeader.fields, test.ShouldEqual, pcdPointColor)
	err = parsePCDHeaderLine("FIELDS x y z", 1, &fakeHeader)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeHeader.fields, test.ShouldEqual, pcdPointOnly)
	err = parsePCDHeaderLine("FIELDS a b c", 1, &fakeHeader)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unsupported pcd fields")
	// SIZE
	_ = parsePCDHeaderLine("FIELDS x y z rgb", 1, &fakeHeader)
	err = parsePCDHeaderLine("SIZE 4 4 4 4", 2, &fakeHeader)
	test.That(t, err, test.ShouldBeNil)
	_ = parsePCDHeaderLine("FIELDS x y z rgb", 1, &fakeHeader)
	err = parsePCDHeaderLine("SIZE 4 4 4", 2, &fakeHeader)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected number of fields")
	// TYPE
	_ = parsePCDHeaderLine("FIELDS x y z rgb", 1, &fakeHeader)
	err = parsePCDHeaderLine("TYPE F F F I", 3, &fakeHeader)
	test.That(t, err, test.ShouldBeNil)
	err = parsePCDHeaderLine("TYPE F F F", 3, &fakeHeader)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected number of fields")
	// COUNT
	_ = parsePCDHeaderLine("FIELDS x y z rgb", 1, &fakeHeader)
	err = parsePCDHeaderLine("COUNT 1 1 1 1", 4, &fakeHeader)
	test.That(t, err, test.ShouldBeNil)
	err = parsePCDHeaderLine("COUNT 1 1 1", 4, &fakeHeader)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected number of fields")
	// WIDTH
	err = parsePCDHeaderLine("WIDTH 3", 5, &fakeHeader)
	test.That(t, err, test.ShouldBeNil)
	err = parsePCDHeaderLine("WIDTH NOTANUM", 5, &fakeHeader)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid WIDTH field")
	// HEIGHT
	err = parsePCDHeaderLine("HEIGHT 1", 6, &fakeHeader)
	test.That(t, err, test.ShouldBeNil)
	err = parsePCDHeaderLine("HEIGHT NOTANUM", 6, &fakeHeader)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid HEIGHT field")
	// VIEWPOINT
	err = parsePCDHeaderLine("VIEWPOINT 0 0 0 1 0 0 0", 7, &fakeHeader)
	test.That(t, err, test.ShouldBeNil)
	err = parsePCDHeaderLine("VIEWPOINT 0 0 0 1 0 0", 7, &fakeHeader)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected number of fields in VIEWPOINT line.")
	// POINTS
	_ = parsePCDHeaderLine("WIDTH 3", 5, &fakeHeader)
	_ = parsePCDHeaderLine("HEIGHT 1", 6, &fakeHeader)
	err = parsePCDHeaderLine("POINTS 3", 8, &fakeHeader)
	test.That(t, err, test.ShouldBeNil)
	err = parsePCDHeaderLine("POINTS NOTANUM", 8, &fakeHeader)
	test.That(t, err.Error(), test.ShouldContainSubstring, "invalid POINTS field")
	err = parsePCDHeaderLine("POINTS 2", 8, &fakeHeader)
	test.That(t, err.Error(), test.ShouldContainSubstring, "POINTS field 2 does not match WIDTH*HEIGHT")
	// DATA
	err = parsePCDHeaderLine("DATA ascii", 9, &fakeHeader)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeHeader.data, test.ShouldEqual, PCDAscii)
	err = parsePCDHeaderLine("DATA binary", 9, &fakeHeader)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeHeader.data, test.ShouldEqual, PCDBinary)
	err = parsePCDHeaderLine("DATA binary_compressed", 9, &fakeHeader)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeHeader.data, test.ShouldEqual, PCDCompressed)
	err = parsePCDHeaderLine("DATA garbage", 9, &fakeHeader)
	test.That(t, err.Error(), test.ShouldContainSubstring, "unsupported data type")
	// WRONG LINE
	err = parsePCDHeaderLine("VERSION 0.7", 1, &fakeHeader)
	test.That(t, err.Error(), test.ShouldContainSubstring, "line is supposed to start with")
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
	data, dataFlag := cloud2.At(-1, -2, 5)
	test.That(t, dataFlag, test.ShouldBeTrue)
	test.That(t, data.HasColor(), test.ShouldBeFalse)
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
	data, dataFlag := cloud2.At(-1, -2, 5)
	test.That(t, dataFlag, test.ShouldBeTrue)
	test.That(t, data.HasColor(), test.ShouldBeTrue)
	r, g, b := data.RGB255()
	test.That(t, r, test.ShouldEqual, 255)
	test.That(t, g, test.ShouldEqual, 1)
	test.That(t, b, test.ShouldEqual, 2)
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
	logger := logging.NewTestLogger(t)
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
	temp, err := os.CreateTemp(t.TempDir(), "*.las")
	test.That(t, err, test.ShouldBeNil)
	defer os.Remove(temp.Name())

	err = WriteToLASFile(cloud, temp.Name())
	test.That(t, err, test.ShouldBeNil)

	nextCloud, err := NewFromFile(temp.Name(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextCloud, test.ShouldResemble, cloud)
}

func createNewPCD(t *testing.T) string {
	t.Helper()

	cloud := NewKDTree()
	test.That(t, cloud.Set(NewVector(-1, -2, 5), NewBasicData()), test.ShouldBeNil)
	test.That(t, cloud.Set(NewVector(582, 12, 0), NewBasicData()), test.ShouldBeNil)
	test.That(t, cloud.Set(NewVector(7, 6, 1), NewBasicData()), test.ShouldBeNil)
	test.That(t, cloud.Size(), test.ShouldEqual, 3)

	var buf bytes.Buffer
	err := ToPCD(cloud, &buf, PCDBinary)
	test.That(t, err, test.ShouldBeNil)
	gotPCD := buf.String()
	test.That(t, gotPCD, test.ShouldContainSubstring, "WIDTH 3\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "HEIGHT 1\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "POINTS 3\n")
	test.That(t, gotPCD, test.ShouldContainSubstring, "DATA binary\n")

	return gotPCD
}

func TestPCDKDTree(t *testing.T) {
	gotPCD := createNewPCD(t)

	cloud2, err := ReadPCDToKDTree(strings.NewReader(gotPCD))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloud2.Size(), test.ShouldEqual, 3)
	gotPt, found := cloud2.At(-1, -2, 5)
	test.That(t, found, test.ShouldBeTrue)
	test.That(t, gotPt, test.ShouldNotBeNil)
}

func TestPCDOctree(t *testing.T) {
	gotPCD := createNewPCD(t)

	basicOct, err := ReadPCDToBasicOctree(strings.NewReader(gotPCD))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, basicOct.Size(), test.ShouldEqual, 3)
	gotPt, found := basicOct.At(-1, -2, 5)
	test.That(t, found, test.ShouldBeTrue)
	test.That(t, gotPt, test.ShouldNotBeNil)
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
