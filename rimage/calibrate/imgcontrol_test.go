package calibrate

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/rimage"
)

func TestGrayAndAvg(t *testing.T) {
	im, _ := rimage.NewImageFromFile(artifact.MustPath("calibrate/chess3.jpeg"))
	im2, _ := MultiplyGrays(MakeGray(im), MakeGray(im))
	got := GetAvg(im2)
	test.That(t, got, test.ShouldEqual, 12122)

	im, _ = rimage.NewImageFromFile(artifact.MustPath("calibrate/chess2.jpeg"))
	im2, _ = MultiplyGrays(MakeGray(im), MakeGray(im))
	got = GetAvg(im2)
	test.That(t, got, test.ShouldEqual, 11744)

	im, _ = rimage.NewImageFromFile(artifact.MustPath("calibrate/chess1.jpeg"))
	im2, _ = MultiplyGrays(MakeGray(im), MakeGray(im))
	got = GetAvg(im2)
	test.That(t, got, test.ShouldEqual, 11839)
}
