package rimage

import (
	"bufio"
	"bytes"
	"image"
	"image/png"
	"math/rand"
	"os"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestDepthMap1(t *testing.T) {
	m, err := ParseDepthMap(artifact.MustPath("rimage/board2.dat.gz"))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.width, test.ShouldEqual, 1280)
	test.That(t, m.height, test.ShouldEqual, 720)

	origHeight := m.GetDepth(300, 300)
	test.That(t, origHeight, test.ShouldEqual, 749)

	buf := bytes.Buffer{}
	_, err = m.WriteTo(&buf)
	test.That(t, err, test.ShouldBeNil)

	m, err = ReadDepthMap(bufio.NewReader(&buf))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.width, test.ShouldEqual, 1280)
	test.That(t, m.height, test.ShouldEqual, 720)
}

func TestDepthMap2(t *testing.T) {
	m, err := ParseDepthMap(artifact.MustPath("rimage/board2.dat.gz"))
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.width, test.ShouldEqual, 1280)
	test.That(t, m.height, test.ShouldEqual, 720)

	origHeight := m.GetDepth(300, 300)
	test.That(t, origHeight, test.ShouldEqual, 749)

	fn := outDir + "/board2-rt.dat.gz"

	err = m.WriteToFile(fn)
	test.That(t, err, test.ShouldBeNil)

	m, err = ParseDepthMap(fn)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, m.width, test.ShouldEqual, 1280)
	test.That(t, m.height, test.ShouldEqual, 720)
}

func TestCloneDepthMap(t *testing.T) {
	m, err := ParseDepthMap(artifact.MustPath("rimage/board2.dat.gz"))
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
	m, err := ParseDepthMap(artifact.MustPath("rimage/depthformat2.dat.gz"))
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

//  1 2              5 3 1 //  1 2               2 4 6
//  3 4  -- 90 cw -> 6 4 2 //  3 4  -- 90 ccw -> 1 3 5
//  5 6                    //  5 6
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
	iwd, err := newImageWithDepth(artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), false)
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
	dm, err := ParseDepthMap(artifact.MustPath("rimage/depthformat2.dat.gz"))
	test.That(b, err, test.ShouldBeNil)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		dm.Rotate90(true)
	}
}

func BenchmarkDepthMapRotate180(b *testing.B) {
	dm, err := ParseDepthMap(artifact.MustPath("rimage/depthformat2.dat.gz"))
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
	iwd, err := newImageWithDepth(artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), false)
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
