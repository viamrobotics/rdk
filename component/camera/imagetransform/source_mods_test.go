package imagetransform

import (
	"context"
	"image"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/component/camera/imagesource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rlog"
)

var outDir string

func init() {
	var err error
	outDir, err = testutils.TempDir("", "rimage_imagetransform")
	if err != nil {
		panic(err)
	}
	rlog.Logger.Debugf("out dir: %q", outDir)
}

func TestRotateSource(t *testing.T) {
	pc, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), true)
	test.That(t, err, test.ShouldBeNil)

	source := &imagesource.StaticSource{ColorImg: pc, DepthImg: pc}
	rs := &rotateImageDepthSource{source}

	rawImage, _, err := rs.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	err = rimage.WriteImageToFile(outDir+"/test_rotate_source.png", rawImage)
	test.That(t, err, test.ShouldBeNil)

	img := rimage.ConvertImage(rawImage)

	for x := 0; x < pc.Color.Width(); x++ {
		p1 := image.Point{x, 0}
		p2 := image.Point{pc.Color.Width() - x - 1, pc.Color.Height() - 1}

		a := pc.Color.Get(p1)
		b := img.Get(p2)

		d := a.Distance(b)
		test.That(t, d, test.ShouldEqual, 0)

		d1 := pc.Depth.Get(p1)
		d2 := rawImage.(*rimage.ImageWithDepth).Depth.Get(p2)

		test.That(t, d1, test.ShouldEqual, d2)
	}
}

func BenchmarkRotate(b *testing.B) {
	pc, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), true)
	test.That(b, err, test.ShouldBeNil)

	source := &imagesource.StaticSource{ColorImg: pc, DepthImg: pc}
	rs := &rotateImageDepthSource{source}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		rs.Next(context.Background())
	}
}
