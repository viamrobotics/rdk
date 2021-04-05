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

func TestSobelFilterGrad(t *testing.T) {
	// circle.png is 300x200, circle is 150 pixels in diameter, centered at (150,100)
	dm, err := loadPictureConvertToDM("data/circle.png")
	if err != nil {
		t.Fatal(err)
	}

	gradients := SobelFilterGrad(dm)
	assert.Equal(t, dm.Height()-2, gradients.Height())
	assert.Equal(t, dm.Width()-2, gradients.Width())
	// reminder: left-handed coordinate system. +x is right, +y is down.
	// (223,100) should have direction 0 degrees
	assert.Equal(t, 0., gradients.GetGradient(223, 100).Direction())
	// (149,173) should have direction 90 degrees
	assert.Equal(t, math.Pi/2., gradients.GetGradient(149, 173).Direction())
	// (76,100) should have direction 180 degrees
	assert.Equal(t, math.Pi, gradients.GetGradient(76, 100).Direction())
	// (149,26) should have direction 270 degrees
	assert.Equal(t, 3.*math.Pi/2., gradients.GetGradient(149, 26).Direction())

	img := gradients.ToPrettyPicture()
	err = writePicture(img, "out/circle_gradient.png")
	if err != nil {
		t.Fatal(err)
	}

}

func TestSobelFilterMat(t *testing.T) {
	// circle.png is 300x200, circle is 150 pixels in diameter, centered at (150,100)
	dm, err := loadPictureConvertToDM("data/circle.png")
	if err != nil {
		t.Fatal(err)
	}

	mags, dirs := SobelFilterMat(dm)
	magH, magW := mags.Dims()
	assert.Equal(t, dm.Height()-2, magH)
	assert.Equal(t, dm.Width()-2, magW)
	// reminder: left-handed coordinate system. +x is right, +y is down.
	// for mat.Dense, y coorindate is first, then x coodinate.
	// (223,100) should have direction 0 degrees
	assert.Equal(t, 0., dirs.At(100, 223))
	// (149,173) should have direction 90 degrees
	assert.Equal(t, math.Pi/2., dirs.At(173, 149))
	// (76,100) should have direction 180 degrees
	assert.Equal(t, math.Pi, dirs.At(100, 76))
	// (149,26) should have direction 270 degrees
	assert.Equal(t, 3.*math.Pi/2., dirs.At(26, 149))

	gradients, err := MakeGradientFieldFromMat(mags, dirs)
	if err != nil {
		t.Fatal(err)
	}
	img := gradients.ToPrettyPicture()
	err = writePicture(img, "out/circle_gradient.png")
}

func BenchmarkSobelFilterGrad(b *testing.B) {
	dm, err := loadPictureConvertToDM("data/shelf_grayscale.png")
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		_ = SobelFilterGrad(dm)
	}
}

func BenchmarkSobelFilterMat(b *testing.B) {
	dm, err := loadPictureConvertToDM("data/shelf_grayscale.png")
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		_, _ = SobelFilterMat(dm)
	}
}
