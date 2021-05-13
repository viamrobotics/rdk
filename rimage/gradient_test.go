package rimage

import (
	"image"
	"image/png"
	"math"
	"os"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/artifact"
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
			test.That(t, magMat.At(y, x), test.ShouldEqual, vf.Get(p).Magnitude())
			test.That(t, dirMat.At(y, x), test.ShouldEqual, vf.Get(p).Direction())
		}
	}
	// turn back into VectorField2D
	vf2, err := VectorField2DFromDense(magMat, dirMat)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, vf2, test.ShouldResemble, &vf)

}

func TestSobelFilter(t *testing.T) {
	// circle.png is 300x200 canvas, circle is 150 pixels in diameter, centered at (150,100)
	dm, err := NewDepthMapFromImageFile(artifact.MustPath("rimage/circle.png"))
	test.That(t, err, test.ShouldBeNil)

	gradients := SobelFilter(dm)
	test.That(t, gradients.Height(), test.ShouldEqual, dm.Height()-2)
	test.That(t, gradients.Width(), test.ShouldEqual, dm.Width()-2)
	// reminder: left-handed coordinate system. +x is right, +y is down.
	// (223,100) is right edge of circle
	test.That(t, gradients.GetVec2D(223, 100).Direction(), test.ShouldEqual, 0.)
	// (149,173) is bottom edge of circle
	test.That(t, gradients.GetVec2D(149, 173).Direction(), test.ShouldEqual, math.Pi/2.)
	// (76,100) is left edge of circle
	test.That(t, gradients.GetVec2D(76, 100).Direction(), test.ShouldEqual, math.Pi)
	// (149,26) is top edge of circle
	test.That(t, gradients.GetVec2D(149, 26).Direction(), test.ShouldEqual, 3.*math.Pi/2.)

	img := gradients.ToPrettyPicture()
	err = writePicture(img, outDir+"/circle_gradient.png")
	test.That(t, err, test.ShouldBeNil)

}

func BenchmarkSobelFilter(b *testing.B) {
	dm, err := NewDepthMapFromImageFile(artifact.MustPath("rimage/shelf_grayscale.png"))
	test.That(b, err, test.ShouldBeNil)
	for i := 0; i < b.N; i++ {
		_ = SobelFilter(dm)
	}
}
