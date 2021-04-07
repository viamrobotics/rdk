package rimage

import (
	"image"
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func loadPictureConvertToDM(p string) (*DepthMap, error) {
	depthImg, err := ReadImageFromFile(p)
	if err != nil {
		return nil, err
	}
	dm, err := ConvertImageToDepthMap(depthImg)
	if err != nil {
		return nil, err
	}
	return dm, nil
}

func writePicture(img image.Image, p string) error {
	file, err := os.Create(p)
	if err != nil {
		return err
	}
	os.MkdirAll("out", 0775)
	defer file.Close()
	png.Encode(file, img)
	return nil
}

func TestVectorFieldToDense(t *testing.T) {
	width, height := 200, 100
	vf := MakeEmptyVectorField2D(width, height)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			mag, dir := getMagnitudeAndDirection(float64(x), float64(y))
			vf.Set(x, y, PolarVec{mag, dir})
		}
	}
	magMat := vf.MagnitudeField()
	dirMat := vf.DirectionField()
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			p := image.Point{x, y}
			assert.Equal(t, vf.Get(p).Magnitude(), magMat.At(y, x))
			assert.Equal(t, vf.Get(p).Direction(), dirMat.At(y, x))
		}
	}

}

func TestSobelFilter(t *testing.T) {
	// circle.png is 300x200 canvas, circle is 150 pixels in diameter, centered at (150,100)
	dm, err := loadPictureConvertToDM("data/circle.png")
	if err != nil {
		t.Fatal(err)
	}

	gradients := SobelFilter(dm)
	assert.Equal(t, dm.Height()-2, gradients.Height())
	assert.Equal(t, dm.Width()-2, gradients.Width())
	// reminder: left-handed coordinate system. +x is right, +y is down.
	// (223,100) is right edge of circle
	assert.Equal(t, 0., gradients.GetPolarVec(223, 100).Direction())
	// (149,173) is bottom edge of circle
	assert.Equal(t, math.Pi/2., gradients.GetPolarVec(149, 173).Direction())
	// (76,100) is left edge of circle
	assert.Equal(t, math.Pi, gradients.GetPolarVec(76, 100).Direction())
	// (149,26) is top edge of circle
	assert.Equal(t, 3.*math.Pi/2., gradients.GetPolarVec(149, 26).Direction())

	img := gradients.ToPrettyPicture()
	err = writePicture(img, "out/circle_gradient.png")
	if err != nil {
		t.Fatal(err)
	}

}

func BenchmarkSobelFilter(b *testing.B) {
	dm, err := loadPictureConvertToDM("data/shelf_grayscale.png")
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		_ = SobelFilter(dm)
	}
}
