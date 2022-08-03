package rimage

import (
	"bytes"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
)

func TestPCRoundTrip(t *testing.T) {
	pc, err := newImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), true)
	test.That(t, err, test.ShouldBeNil)

	same := func(other *imageWithDepth) {
		test.That(t, other.Color.Width(), test.ShouldEqual, pc.Color.Width())
		test.That(t, other.Color.Height(), test.ShouldEqual, pc.Color.Height())
		test.That(t, other.Depth.Width(), test.ShouldEqual, pc.Depth.Width())
		test.That(t, other.Depth.Height(), test.ShouldEqual, pc.Depth.Height())

		test.That(t, other.Depth.GetDepth(111, 111), test.ShouldEqual, pc.Depth.GetDepth(111, 111))
	}

	fn := outDir + "/roundtrip1.both.gz"
	err = pc.WriteTo(fn)
	test.That(t, err, test.ShouldBeNil)

	img2, dm2, err := ReadBothFromFile(fn)
	test.That(t, err, test.ShouldBeNil)
	pc2 := &imageWithDepth{img2, dm2, true}

	same(pc2)

	var buf bytes.Buffer
	test.That(t, pc.RawBytesWrite(&buf), test.ShouldBeNil)

	pc3, err := imageWithDepthFromRawBytes(pc.Color.Width(), pc.Color.Height(), buf.Bytes())
	test.That(t, err, test.ShouldBeNil)
	same(pc3)
}

func TestCloneImageWithDepth(t *testing.T) {
	iwd, err := newImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), true)
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

func TestImageWithDepthFromImages(t *testing.T) {
	iwd, err := newImageWithDepthFromImages(
		artifact.MustPath("rimage/shelf_color.png"), artifact.MustPath("rimage/shelf_grayscale.png"), false)
	test.That(t, err, test.ShouldBeNil)

	err = iwd.WriteTo(outDir + "/shelf.both.gz")
	test.That(t, err, test.ShouldBeNil)
}

func TestImageToDepthMap(t *testing.T) {
	iwd, err := newImageWithDepth(
		artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	// convert to gray16 image
	depthImage := iwd.Depth.ToGray16Picture()
	// convert back
	dmFromImage := imageToDepthMap(depthImage)
	// tests
	test.That(t, iwd.Depth.Height(), test.ShouldEqual, dmFromImage.Height())
	test.That(t, iwd.Depth.Width(), test.ShouldEqual, dmFromImage.Width())
	test.That(t, iwd.Depth, test.ShouldResemble, dmFromImage)
}

func TestConvertToDepthMap(t *testing.T) {
	iwd, err := newImageWithDepth(
		artifact.MustPath("rimage/board2.png"), artifact.MustPath("rimage/board2.dat.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	// convert to gray16 image
	depthImage := iwd.Depth.ToGray16Picture()

	// case 1
	dm1, err := ConvertImageToDepthMap(iwd)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, iwd.Depth, test.ShouldEqual, dm1)
	// case 2
	dm2, err := ConvertImageToDepthMap(depthImage)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, iwd.Depth, test.ShouldResemble, dm2)
	// default - should return error
	badType := iwd.Color
	_, err = ConvertImageToDepthMap(badType)
	test.That(t, err, test.ShouldNotBeNil)
}
