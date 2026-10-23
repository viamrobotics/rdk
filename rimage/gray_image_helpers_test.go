package rimage

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestGrayAndAvg(t *testing.T) {
	t.Parallel()
	im, _ := NewImageFromFile(artifact.MustPath("calibrate/chess3.jpeg"))
	im2, _ := MultiplyGrays(MakeGray(im), MakeGray(im))
	got := GetGrayAvg(im2)
	test.That(t, got, test.ShouldBeBetweenOrEqual, 12121, 12123)

	im, _ = NewImageFromFile(artifact.MustPath("calibrate/chess2.jpeg"))
	im2, _ = MultiplyGrays(MakeGray(im), MakeGray(im))
	got = GetGrayAvg(im2)
	test.That(t, got, test.ShouldBeBetweenOrEqual, 11743, 11745)

	im, _ = NewImageFromFile(artifact.MustPath("calibrate/chess1.jpeg"))
	im2, _ = MultiplyGrays(MakeGray(im), MakeGray(im))
	got = GetGrayAvg(im2)
	test.That(t, got, test.ShouldBeBetweenOrEqual, 11838, 11840)
}
