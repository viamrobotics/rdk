package rimage

import (
	"image"
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.viam.com/robotcore/artifact"
)

func writePicture(img image.Image, p string) error {
	file, err := os.Create(p)
	if err != nil {
		return err
	}
	defer file.Close()
	png.Encode(file, img)
	return nil
}

func TestVectorFieldToDenseAndBack(t *testing.T) {
	width, height := 200, 100
	vf := MakeEmptyVectorField2D(width, height)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			mag, dir := getMagnitudeAndDirection(float64(x), float64(y))
			vf.Set(x, y, Vec2D{mag, dir})
		}
	}
	// turn into mat.Dense
	magMat := vf.MagnitudeField()
	dirMat := vf.DirectionField()
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			p := image.Point{x, y}
			assert.Equal(t, vf.Get(p).Magnitude(), magMat.At(y, x))
			assert.Equal(t, vf.Get(p).Direction(), dirMat.At(y, x))
		}
	}
	// turn back into VectorField2D
	vf2, err := VectorField2DFromDense(magMat, dirMat)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, &vf, vf2)

}

func TestSobelFilter(t *testing.T) {
	// circle.png is 300x200 canvas, circle is 150 pixels in diameter, centered at (150,100)
	dm, err := NewDepthMapFromImageFile(artifact.MustPath("rimage/circle.png"))
	if err != nil {
		t.Fatal(err)
	}

	gradients := SobelFilter(dm)
	assert.Equal(t, dm.Height()-2, gradients.Height())
	assert.Equal(t, dm.Width()-2, gradients.Width())
	// reminder: left-handed coordinate system. +x is right, +y is down.
	// (223,100) is right edge of circle
	assert.Equal(t, 0., gradients.GetVec2D(223, 100).Direction())
	// (149,173) is bottom edge of circle
	assert.Equal(t, math.Pi/2., gradients.GetVec2D(149, 173).Direction())
	// (76,100) is left edge of circle
	assert.Equal(t, math.Pi, gradients.GetVec2D(76, 100).Direction())
	// (149,26) is top edge of circle
	assert.Equal(t, 3.*math.Pi/2., gradients.GetVec2D(149, 26).Direction())

	img := gradients.ToPrettyPicture()
	err = writePicture(img, outDir+"/circle_gradient.png")
	if err != nil {
		t.Fatal(err)
	}

}

func BenchmarkSobelFilter(b *testing.B) {
	dm, err := NewDepthMapFromImageFile(artifact.MustPath("rimage/shelf_grayscale.png"))
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		_ = SobelFilter(dm)
	}
}
