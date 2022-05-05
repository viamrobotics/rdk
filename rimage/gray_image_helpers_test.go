package rimage

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestGrayAndAvg(t *testing.T) {
	im, _ := NewImageFromFile(artifact.MustPath("calibrate/chess3.jpeg"))
	im2, _ := MultiplyGrays(MakeGray(im), MakeGray(im))
	got := GetGrayAvg(im2)
	test.That(t, got, test.ShouldEqual, 12122)

	im, _ = NewImageFromFile(artifact.MustPath("calibrate/chess2.jpeg"))
	im2, _ = MultiplyGrays(MakeGray(im), MakeGray(im))
	got = GetGrayAvg(im2)
	test.That(t, got, test.ShouldEqual, 11744)

	im, _ = NewImageFromFile(artifact.MustPath("calibrate/chess1.jpeg"))
	im2, _ = MultiplyGrays(MakeGray(im), MakeGray(im))
	got = GetGrayAvg(im2)
	test.That(t, got, test.ShouldEqual, 11839)
}
