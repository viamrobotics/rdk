package rimage

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestCloneImageWithDepth(t *testing.T) {
	iwd, err := newImageWithDepth(
		context.Background(),
		artifact.MustPath("rimage/board1.png"),
		artifact.MustPath("rimage/board1.dat.gz"),
		true,
	)
	test.That(t, err, test.ShouldBeNil)

	ii := cloneToImageWithDepth(iwd)
	for y := 0; y < ii.Height(); y++ {
		for x := 0; x < ii.Width(); x++ {
			test.That(t, ii.Depth.GetDepth(x, y), test.ShouldResemble, iwd.Depth.GetDepth(x, y))
			test.That(t, ii.Color.GetXY(x, y), test.ShouldResemble, iwd.Color.GetXY(x, y))
		}
	}
	test.That(t, ii.IsAligned(), test.ShouldEqual, iwd.IsAligned())
}

func TestImageToDepthMap(t *testing.T) {
	iwd, err := newImageWithDepth(context.Background(),
		artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	// convert to gray16 image
	depthImage := iwd.Depth.ToGray16Picture()
	// convert back
	dmFromImage, err := ConvertImageToDepthMap(context.Background(), depthImage)
	test.That(t, err, test.ShouldBeNil)
	// tests
	test.That(t, iwd.Depth.Height(), test.ShouldEqual, dmFromImage.Height())
	test.That(t, iwd.Depth.Width(), test.ShouldEqual, dmFromImage.Width())
	test.That(t, iwd.Depth, test.ShouldResemble, dmFromImage)
}

func TestConvertToDepthMap(t *testing.T) {
	iwd, err := newImageWithDepth(context.Background(),
		artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	// convert to gray16 image
	depthImage := iwd.Depth.ToGray16Picture()

	// case 1
	dm1, err := ConvertImageToDepthMap(context.Background(), iwd)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, iwd.Depth, test.ShouldEqual, dm1)
	// case 2
	dm2, err := ConvertImageToDepthMap(context.Background(), depthImage)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, iwd.Depth, test.ShouldResemble, dm2)
	// default - should return error
	badType := iwd.Color
	_, err = ConvertImageToDepthMap(context.Background(), badType)
	test.That(t, err, test.ShouldNotBeNil)
}
