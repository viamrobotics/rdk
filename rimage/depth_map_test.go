package rimage

import (
	"bufio"
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"os"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/testutils"
)

func TestRawDepthMap(t *testing.T) {
	m, err := ParseRawDepthMap(artifact.MustPath("rimage/board2.dat.gz"))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.Width(), test.ShouldEqual, 1280)
	test.That(t, m.Height(), test.ShouldEqual, 720)
	origHeight := m.GetDepth(300, 300)
	test.That(t, origHeight, test.ShouldEqual, 749)

	buf := bytes.Buffer{}
	_, err = WriteRawDepthMapTo(m, &buf)
	test.That(t, err, test.ShouldBeNil)

	m, err = ReadDepthMap(bufio.NewReader(&buf))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.Width(), test.ShouldEqual, 1280)
	test.That(t, m.Height(), test.ShouldEqual, 720)
	origHeight = m.GetDepth(300, 300)
	test.That(t, origHeight, test.ShouldEqual, 749)

	fn := outDir + "/board2-rt.dat.gz"

	err = WriteRawDepthMapToFile(m, fn)
	test.That(t, err, test.ShouldBeNil)

	m, err = ParseRawDepthMap(fn)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.Width(), test.ShouldEqual, 1280)
	test.That(t, m.Height(), test.ShouldEqual, 720)
	origHeight = m.GetDepth(300, 300)
	test.That(t, origHeight, test.ShouldEqual, 749)
}

func TestDepthMap(t *testing.T) {
	m, err := NewDepthMapFromFile(context.Background(), artifact.MustPath("rimage/board2_gray.png"))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.Width(), test.ShouldEqual, 1280)
	test.That(t, m.Height(), test.ShouldEqual, 720)
	origHeight := m.GetDepth(300, 300)
	test.That(t, origHeight, test.ShouldEqual, 749)

	buf := bytes.Buffer{}
	err = m.WriteToBuf(&buf)
	test.That(t, err, test.ShouldBeNil)

	img, _, err := image.Decode(bufio.NewReader(&buf))
	test.That(t, err, test.ShouldBeNil)
	m, err = ConvertImageToDepthMap(context.Background(), img)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.Width(), test.ShouldEqual, 1280)
	test.That(t, m.Height(), test.ShouldEqual, 720)
	origHeight = m.GetDepth(300, 300)
	test.That(t, origHeight, test.ShouldEqual, 749)

	fn := outDir + "/board2-rt.png"

	err = WriteImageToFile(fn, m)
	test.That(t, err, test.ShouldBeNil)

	m, err = NewDepthMapFromFile(context.Background(), fn)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.Width(), test.ShouldEqual, 1280)
	test.That(t, m.Height(), test.ShouldEqual, 720)
	origHeight = m.GetDepth(300, 300)
	test.That(t, origHeight, test.ShouldEqual, 749)
}

func TestCloneDepthMap(t *testing.T) {
	m, err := NewDepthMapFromFile(context.Background(), artifact.MustPath("rimage/board2_gray.png"))
	test.That(t, err, test.ShouldBeNil)

	mm := m.Clone()
	for y := 0; y < m.Height(); y++ {
		for x := 0; x < m.Width(); x++ {
			test.That(t, mm.GetDepth(x, y), test.ShouldResemble, m.GetDepth(x, y))
		}
	}
	mm.Set(0, 0, Depth(5000))
	test.That(t, mm.GetDepth(0, 0), test.ShouldNotResemble, m.GetDepth(0, 0))
}

func TestDepthMapNewFormat(t *testing.T) {
	m, err := ParseRawDepthMap(artifact.MustPath("rimage/depthformat2.dat.gz"))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.width, test.ShouldEqual, 1280)
	test.That(t, m.height, test.ShouldEqual, 720)

	numZero := 0

	for x := 0; x < m.width; x++ {
		d := m.GetDepth(x, m.height-1)
		if d == 0 {
			numZero++
		} else {
			test.That(t, d, test.ShouldBeBetween, 100, 5000)
		}
	}

	test.That(t, numZero, test.ShouldBeBetween, 0, m.width)
}

// 1 2              5 3 1 //  1 2               2 4 6
// 3 4  -- 90 cw -> 6 4 2 //  3 4  -- 90 ccw -> 1 3 5
// 5 6                    //  5 6.
func TestDepthRotate90(t *testing.T) {
	dm := NewEmptyDepthMap(2, 3)
	dm.Set(0, 0, 1)
	dm.Set(1, 0, 2)
	dm.Set(0, 1, 3)
	dm.Set(1, 1, 4)
	dm.Set(0, 2, 5)
	dm.Set(1, 2, 6)

	dm2 := dm.Rotate90(true)
	test.That(t, dm2.Height(), test.ShouldEqual, 2)
	test.That(t, dm2.Width(), test.ShouldEqual, 3)
	test.That(t, dm2.GetDepth(0, 0), test.ShouldEqual, Depth(5))
	test.That(t, dm2.GetDepth(2, 1), test.ShouldEqual, Depth(2))
	dm3 := dm.Rotate90(false)
	test.That(t, dm3.Height(), test.ShouldEqual, 2)
	test.That(t, dm3.Width(), test.ShouldEqual, 3)
	test.That(t, dm3.GetDepth(0, 0), test.ShouldEqual, Depth(2))
	test.That(t, dm3.GetDepth(2, 1), test.ShouldEqual, Depth(5))
}

func TestToGray16Picture(t *testing.T) {
	iwd, err := newImageWithDepth(
		context.Background(),
		artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), false,
	)
	test.That(t, err, test.ShouldBeNil)
	gimg := iwd.Depth.ToGray16Picture()

	test.That(t, gimg.Bounds().Max.X, test.ShouldEqual, iwd.Depth.Width())
	test.That(t, gimg.Bounds().Max.Y, test.ShouldEqual, iwd.Depth.Height())

	file, err := os.Create(outDir + "/board2_gray.png")
	test.That(t, err, test.ShouldBeNil)
	defer file.Close()
	png.Encode(file, gimg)
}

//nolint:dupl
func makeImagesForSubImageTest(ori, crop image.Rectangle) (*Image, *Image) {
	oriWidth, oriHeight := ori.Max.X-ori.Min.X, ori.Max.Y-ori.Min.Y
	overlap := ori.Intersect(crop)
	cropWidth, cropHeight := overlap.Max.X-overlap.Min.X, overlap.Max.Y-overlap.Min.Y
	oriData := make([]Color, 0, oriWidth*oriHeight)
	cropData := make([]Color, 0, cropWidth*cropHeight)
	i := Color(0)
	for y := ori.Min.Y; y < ori.Max.Y; y++ {
		for x := ori.Min.X; x < ori.Max.X; x++ {
			oriData = append(oriData, i)
			if x >= overlap.Min.X && x < overlap.Max.X && y >= overlap.Min.Y && y < overlap.Max.Y {
				cropData = append(cropData, i)
			}
			i++
		}
	}
	if crop.Empty() {
		return &Image{data: oriData, width: oriWidth, height: oriHeight}, &Image{}
	}
	return &Image{data: oriData, width: oriWidth, height: oriHeight}, &Image{data: cropData, width: cropWidth, height: cropHeight}
}

//nolint:dupl
func makeDepthMapsForSubImageTest(ori, crop image.Rectangle) (*DepthMap, *DepthMap) {
	oriWidth, oriHeight := ori.Max.X-ori.Min.X, ori.Max.Y-ori.Min.Y
	overlap := ori.Intersect(crop)
	cropWidth, cropHeight := overlap.Max.X-overlap.Min.X, overlap.Max.Y-overlap.Min.Y
	oriData := make([]Depth, 0, oriWidth*oriHeight)
	cropData := make([]Depth, 0, cropWidth*cropHeight)
	i := Depth(0)
	for y := ori.Min.Y; y < ori.Max.Y; y++ {
		for x := ori.Min.X; x < ori.Max.X; x++ {
			oriData = append(oriData, i)
			if x >= overlap.Min.X && x < overlap.Max.X && y >= overlap.Min.Y && y < overlap.Max.Y {
				cropData = append(cropData, i)
			}
			i++
		}
	}
	if crop.Empty() {
		return &DepthMap{data: oriData, width: oriWidth, height: oriHeight}, &DepthMap{}
	}
	return &DepthMap{width: oriWidth, height: oriHeight, data: oriData}, &DepthMap{width: cropWidth, height: cropHeight, data: cropData}
}

func TestSubImage(t *testing.T) {
	type subImages struct{ Original, Crop image.Rectangle }
	tests := []subImages{
		{image.Rect(0, 0, 100, 75), image.Rect(0, 0, 100, 75)},      // crop of the same size
		{image.Rect(0, 0, 100, 75), image.Rect(0, 0, 10, 5)},        // crop upper left
		{image.Rect(0, 0, 100, 75), image.Rect(90, 70, 100, 75)},    // crop lower right
		{image.Rect(0, 0, 100, 75), image.Rect(30, 40, 35, 45)},     // crop middle
		{image.Rect(0, 0, 100, 75), image.Rect(0, 0, 100, 2)},       // crop top
		{image.Rect(0, 0, 100, 75), image.Rect(0, 72, 100, 75)},     // crop bottom
		{image.Rect(0, 0, 100, 75), image.Rect(98, 0, 100, 75)},     // crop right
		{image.Rect(0, 0, 100, 75), image.Rect(0, 0, 2, 75)},        // crop left
		{image.Rect(0, 0, 100, 75), image.Rect(95, 70, 105, 80)},    // crop is not a full subset
		{image.Rect(0, 0, 100, 75), image.Rect(200, 200, 300, 300)}, // out of bounds
		{image.Rect(0, 0, 100, 75), image.Rectangle{}},              // empty
	}
	for _, rec := range tests {
		originalImg, expectedCrop := makeImagesForSubImageTest(rec.Original, rec.Crop)
		crop := originalImg.SubImage(rec.Crop)
		test.That(t, crop, test.ShouldResemble, expectedCrop)
	}
	for _, rec := range tests {
		originalDM, expectedCrop := makeDepthMapsForSubImageTest(rec.Original, rec.Crop)
		crop := originalDM.SubImage(rec.Crop)
		test.That(t, crop, test.ShouldResemble, expectedCrop)
	}
}

func BenchmarkDepthMapRotate90(b *testing.B) {
	dm, err := ParseRawDepthMap(artifact.MustPath("rimage/depthformat2.dat.gz"))
	test.That(b, err, test.ShouldBeNil)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		dm.Rotate90(true)
	}
}

func BenchmarkDepthMapRotate180(b *testing.B) {
	dm, err := ParseRawDepthMap(artifact.MustPath("rimage/depthformat2.dat.gz"))
	test.That(b, err, test.ShouldBeNil)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		dm.Rotate180()
	}
}

func TestDepthMapStats(t *testing.T) {
	dm := NewEmptyDepthMap(3, 3)
	for x := 0; x < 3; x++ {
		for y := 0; y < 3; y++ {
			dm.Set(x, y, Depth((x*10)+y))
		}
	}

	d, a := dm.AverageDepthAndStats(image.Point{1, 1}, 0)
	test.That(t, d, test.ShouldEqual, 11.0)
	test.That(t, a, test.ShouldEqual, 0.0)

	d, a = dm.AverageDepthAndStats(image.Point{1, 1}, 1)
	test.That(t, d, test.ShouldEqual, 12.375)
	test.That(t, a, test.ShouldEqual, 6.46875)

	d, a = dm.AverageDepthAndStats(image.Point{3, 3}, 1)
	test.That(t, d, test.ShouldEqual, 22.0)
	test.That(t, a, test.ShouldEqual, 0.0)

	img := dm.InterestingPixels(5)
	test.That(t, img.GrayAt(1, 1).Y, test.ShouldEqual, uint8(255))

	img = dm.InterestingPixels(10)
	test.That(t, img.GrayAt(1, 1).Y, test.ShouldEqual, uint8(0))
}

func TestDepthMap_ConvertDepthMapToLuminanceFloat(t *testing.T) {
	iwd, err := newImageWithDepth(
		context.Background(),
		artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), false,
	)
	test.That(t, err, test.ShouldBeNil)
	fimg := iwd.Depth.ConvertDepthMapToLuminanceFloat()
	nRows, nCols := fimg.Dims()
	// test dimensions
	test.That(t, nCols, test.ShouldEqual, iwd.Depth.Width())
	test.That(t, nRows, test.ShouldEqual, iwd.Depth.Height())
	// test values
	// select random pixel
	x, y := rand.Intn(nCols), rand.Intn(nRows)
	test.That(t, fimg.At(y, x), test.ShouldEqual, float64(iwd.Depth.GetDepth(x, y)))
}

func TestGray8DepthConversion(t *testing.T) {
	testGray := image.NewGray(image.Rect(0, 0, 5, 10))
	width, height := testGray.Bounds().Dx(), testGray.Bounds().Dy()
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			testGray.SetGray(x, y, color.Gray{Y: uint8(y + x)})
		}
	}
	// convert to depthMap
	dm, err := ConvertImageToDepthMap(context.Background(), testGray)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dm.Bounds(), test.ShouldResemble, testGray.Bounds())
	for x := 0; x < dm.Bounds().Dx(); x++ {
		for y := 0; y < dm.Bounds().Dy(); y++ {
			test.That(t, int(dm.GetDepth(x, y)), test.ShouldEqual, int(testGray.GrayAt(x, y).Y))
		}
	}
	// convert to gray16
	g16, err := ConvertImageToGray16(testGray)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g16.Bounds(), test.ShouldResemble, testGray.Bounds())
	for x := 0; x < g16.Bounds().Dx(); x++ {
		for y := 0; y < g16.Bounds().Dy(); y++ {
			test.That(t, int(g16.Gray16At(x, y).Y), test.ShouldEqual, int(testGray.GrayAt(x, y).Y))
		}
	}
}

func TestDepthColorModel(t *testing.T) {
	dm := NewEmptyDepthMap(1, 1)
	// DepthMap Color model should convert to Gray16
	gray := color.Gray16{Y: 5}
	convGray := dm.ColorModel().Convert(gray)
	test.That(t, convGray, test.ShouldHaveSameTypeAs, gray)
	test.That(t, convGray.(color.Gray16).Y, test.ShouldEqual, gray.Y)
	// test Gray8
	gray8 := color.Gray{Y: math.MaxUint8}
	convGray = dm.ColorModel().Convert(gray8)
	test.That(t, convGray, test.ShouldHaveSameTypeAs, gray)
	test.That(t, convGray.(color.Gray16).Y, test.ShouldEqual, math.MaxUint16)
	gray8 = color.Gray{Y: 24} // copies the 8 bits, to 16 bits: 0001 1000 -> 0001 1000 0001 1000
	convGray = dm.ColorModel().Convert(gray8)
	test.That(t, convGray, test.ShouldHaveSameTypeAs, gray)
	test.That(t, convGray.(color.Gray16).Y, test.ShouldEqual, 6168)
	// do it directly in binary for clarity
	gray8 = color.Gray{Y: 0b01101100}
	convGray = dm.ColorModel().Convert(gray8)
	test.That(t, convGray, test.ShouldHaveSameTypeAs, gray)
	test.That(t, convGray.(color.Gray16).Y, test.ShouldEqual, 0b0110110001101100)
	// test max value
	maxGray := color.Gray16{Y: math.MaxUint16}
	convGray = dm.ColorModel().Convert(maxGray)
	test.That(t, convGray, test.ShouldHaveSameTypeAs, gray)
	test.That(t, convGray.(color.Gray16).Y, test.ShouldEqual, maxGray.Y)
	// 8 bit color gets copied into the next byte
	rgba8 := color.NRGBA{24, 24, 24, math.MaxUint8}
	convGray = dm.ColorModel().Convert(rgba8)
	test.That(t, convGray, test.ShouldHaveSameTypeAs, gray)
	test.That(t, convGray.(color.Gray16).Y, test.ShouldEqual, 6168)
}

func TestDepthMapEncoding(t *testing.T) {
	m, err := NewDepthMapFromFile(context.Background(), artifact.MustPath("rimage/fakeDM.vnd.viam.dep"))
	test.That(t, err, test.ShouldBeNil)

	// Test values at points of DepthMap
	// This example DepthMap (fakeDM) was made such that Depth(x,y) = x*y
	test.That(t, m.Width(), test.ShouldEqual, 20)
	test.That(t, m.Height(), test.ShouldEqual, 10)
	testPt1 := m.GetDepth(13, 3)
	test.That(t, testPt1, test.ShouldEqual, 39)
	testPt2 := m.GetDepth(10, 6)
	test.That(t, testPt2, test.ShouldEqual, 60)

	// Save DepthMap BYTES to a file
	buf := bytes.Buffer{}
	err = m.WriteToBuf(&buf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, buf.Bytes(), test.ShouldNotBeNil)
	outDir := testutils.TempDirT(t, "", "rimage")
	saveTo := outDir + "/grayboard_bytes.vnd.viam.dep"
	err = WriteRawDepthMapToFile(m, saveTo)
	test.That(t, err, test.ShouldBeNil)

	newM, err := NewDepthMapFromFile(context.Background(), saveTo)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, newM.Bounds().Dx(), test.ShouldEqual, 20)
	test.That(t, newM.Bounds().Dy(), test.ShouldEqual, 10)
	testPtA := newM.GetDepth(13, 3)
	test.That(t, testPtA, test.ShouldEqual, 39)
	testPtB := newM.GetDepth(10, 6)
	test.That(t, testPtB, test.ShouldEqual, 60)
}
