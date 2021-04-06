package rimage

import (
	"bufio"
	"bytes"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDepthMap1(t *testing.T) {
	m, err := ParseDepthMap("data/board2.dat.gz")
	if err != nil {
		t.Fatal(err)
	}
	m.Smooth()

	if m.width != 1280 {
		t.Errorf("wrong width")
	}
	if m.height != 720 {
		t.Errorf("wrong height")
	}

	origHeight := m.GetDepth(300, 300)
	if origHeight != 749 {
		t.Errorf("wrong depth %v", m.GetDepth(300, 300))
	}

	buf := bytes.Buffer{}
	err = m.WriteTo(&buf)
	if err != nil {
		t.Fatal(err)
	}

	m, err = ReadDepthMap(bufio.NewReader(&buf))
	if err != nil {
		t.Fatal(err)
	}
	if m.width != 1280 {
		t.Errorf("wrong width")
	}
	if m.height != 720 {
		t.Errorf("wrong height")
	}
	if origHeight != 749 {
		t.Errorf("wrong depth")
	}

}

func TestDepthMap2(t *testing.T) {
	m, err := ParseDepthMap("data/board2.dat.gz")
	if err != nil {
		t.Fatal(err)
	}
	m.Smooth()

	if m.width != 1280 {
		t.Errorf("wrong width")
	}
	if m.height != 720 {
		t.Errorf("wrong height")
	}

	origHeight := m.GetDepth(300, 300)
	if origHeight != 749 {
		t.Errorf("wrong depth %v", m.GetDepth(300, 300))
	}

	os.MkdirAll("out", 0775)

	fn := "out/board2-rt.dat.gz"

	err = m.WriteToFile(fn)
	if err != nil {
		t.Fatal(err)
	}

	m, err = ParseDepthMap(fn)
	if err != nil {
		t.Fatal(err)
	}
	if m.width != 1280 {
		t.Errorf("wrong width")
	}
	if m.height != 720 {
		t.Errorf("wrong height")
	}
	if origHeight != 749 {
		t.Errorf("wrong depth")
	}

}

func TestDepthMapNewFormat(t *testing.T) {
	m, err := ParseDepthMap("data/depthformat2.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	if m.width != 1280 || m.height != 720 {
		t.Errorf("width and height wrong %v %v", m.width, m.height)
	}

	numZero := 0

	for x := 0; x < m.width; x++ {
		d := m.GetDepth(x, m.height-1)
		if d == 0 {
			numZero = numZero + 1
		} else {
			if d < 100 || d > 5000 {
				t.Errorf("weird depth %v", d)
			}
		}

	}

	if numZero == 0 || numZero >= m.width {
		t.Errorf("numZero wrong %v", numZero)
	}
}

func TestDepthRotate90(t *testing.T) {
	dm := NewEmptyDepthMap(2, 2)
	dm.Set(0, 0, 1)
	dm.Set(1, 0, 2)
	dm.Set(0, 1, 3)
	dm.Set(1, 1, 4)

	dm2 := dm.Rotate90(true)

	assert.Equal(t, Depth(1), dm2.GetDepth(0, 0))
}

func TestToGray16Picture(t *testing.T) {
	iwd, err := NewImageWithDepth("data/board2.png", "data/board2.dat.gz")
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll("out", 0775)
	gimg := iwd.Depth.ToGray16Picture()

	assert.Equal(t, iwd.Depth.Width(), gimg.Bounds().Max.X)
	assert.Equal(t, iwd.Depth.Height(), gimg.Bounds().Max.Y)

	file, err := os.Create("out/board2_gray.png")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	png.Encode(file, gimg)
}

func makeImagesForSubImageTest(ori, crop image.Rectangle) (Image, Image) {
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
			i = i + 1
		}
	}
	return Image{data: oriData, width: oriWidth, height: oriHeight}, Image{data: cropData, width: cropWidth, height: cropHeight}

}

func makeDepthMapsForSubImageTest(ori, crop image.Rectangle) (DepthMap, DepthMap) {
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
			i = i + 1
		}
	}
	return DepthMap{width: oriWidth, height: oriHeight, data: oriData}, DepthMap{width: cropWidth, height: cropHeight, data: cropData}

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
	}
	for _, rec := range tests {
		originalImg, expectedCrop := makeImagesForSubImageTest(rec.Original, rec.Crop)
		crop := originalImg.SubImage(rec.Crop)
		assert.Equal(t, expectedCrop, crop)
	}
	for _, rec := range tests {
		originalDM, expectedCrop := makeDepthMapsForSubImageTest(rec.Original, rec.Crop)
		crop := originalDM.SubImage(rec.Crop)
		assert.Equal(t, expectedCrop, crop)
	}
}

func BenchmarkDepthMapRotate90(b *testing.B) {
	dm, err := ParseDepthMap("data/depthformat2.dat.gz")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		dm.Rotate90(true)
	}

}

func BenchmarkDepthMapRotate180(b *testing.B) {
	dm, err := ParseDepthMap("data/depthformat2.dat.gz")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		dm.Rotate180()
	}

}
