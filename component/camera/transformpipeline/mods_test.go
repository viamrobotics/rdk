package transformpipeline

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/camera/videosource"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rlog"
)

var outDir string

func init() {
	var err error
	outDir, err = testutils.TempDir("", "camera_transformpipeline")
	if err != nil {
		panic(err)
	}
	rlog.Logger.Debugf("out dir: %q", outDir)
}

//nolint:dupl
func TestResizeColor(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1.png"))
	test.That(t, err, test.ShouldBeNil)

	am := config.AttributeMap{
		"height": 200,
		"width":  300,
	}
	source := gostream.NewVideoSource(&videosource.StaticSource{ColorImg: img}, prop.Video{})
	out, _, err := camera.ReadImage(context.Background(), source)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, out.Bounds().Dy(), test.ShouldEqual, 720)

	rs, err := newResizeTransform(source, camera.ColorStream, am)
	test.That(t, err, test.ShouldBeNil)
	out, _, err = camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Bounds().Dx(), test.ShouldEqual, 300)
	test.That(t, out.Bounds().Dy(), test.ShouldEqual, 200)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(t, source.Close(context.Background()), test.ShouldBeNil)
}

//nolint:dupl
func TestResizeDepth(t *testing.T) {
	img, err := rimage.NewDepthMapFromFile(artifact.MustPath("rimage/board1_gray.png"))
	test.That(t, err, test.ShouldBeNil)

	am := config.AttributeMap{
		"height": 400,
		"width":  600,
	}
	source := gostream.NewVideoSource(&videosource.StaticSource{DepthImg: img}, prop.Video{})
	out, _, err := camera.ReadImage(context.Background(), source)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, out.Bounds().Dy(), test.ShouldEqual, 720)

	rs, err := newResizeTransform(source, camera.DepthStream, am)
	test.That(t, err, test.ShouldBeNil)
	out, _, err = camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Bounds().Dx(), test.ShouldEqual, 600)
	test.That(t, out.Bounds().Dy(), test.ShouldEqual, 400)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(t, source.Close(context.Background()), test.ShouldBeNil)
}

func TestRotateColorSource(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1.png"))
	test.That(t, err, test.ShouldBeNil)

	source := gostream.NewVideoSource(&videosource.StaticSource{ColorImg: img}, prop.Video{})
	rs, err := newRotateTransform(source, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)

	rawImage, _, err := camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)

	err = rimage.WriteImageToFile(outDir+"/test_rotate_color_source.png", rawImage)
	test.That(t, err, test.ShouldBeNil)

	img2 := rimage.ConvertImage(rawImage)

	for x := 0; x < img.Width(); x++ {
		p1 := image.Point{x, 0}
		p2 := image.Point{img.Width() - x - 1, img.Height() - 1}

		a := img.Get(p1)
		b := img2.Get(p2)

		d := a.Distance(b)
		test.That(t, d, test.ShouldEqual, 0)
	}

	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(t, source.Close(context.Background()), test.ShouldBeNil)
}

func TestRotateDepthSource(t *testing.T) {
	pc, err := rimage.NewDepthMapFromFile(artifact.MustPath("rimage/board1.dat.gz"))
	test.That(t, err, test.ShouldBeNil)

	source := gostream.NewVideoSource(&videosource.StaticSource{DepthImg: pc}, prop.Video{})
	rs, err := newRotateTransform(source, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)

	rawImage, _, err := camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)

	err = rimage.WriteImageToFile(outDir+"/test_rotate_depth_source.png", rawImage)
	test.That(t, err, test.ShouldBeNil)

	dm, err := rimage.ConvertImageToDepthMap(rawImage)
	test.That(t, err, test.ShouldBeNil)

	for x := 0; x < pc.Width(); x++ {
		p1 := image.Point{x, 0}
		p2 := image.Point{pc.Width() - x - 1, pc.Height() - 1}

		d1 := pc.Get(p1)
		d2 := dm.Get(p2)

		test.That(t, d1, test.ShouldEqual, d2)
	}

	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(t, source.Close(context.Background()), test.ShouldBeNil)
}

//nolint:dupl
func BenchmarkColorRotate(b *testing.B) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1.png"))
	test.That(b, err, test.ShouldBeNil)

	source := gostream.NewVideoSource(&videosource.StaticSource{ColorImg: img}, prop.Video{})
	cam, err := camera.NewFromSource(source, nil)
	test.That(b, err, test.ShouldBeNil)
	rs, err := newRotateTransform(cam, camera.ColorStream)
	test.That(b, err, test.ShouldBeNil)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		camera.ReadImage(context.Background(), rs)
	}
	test.That(b, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(b, source.Close(context.Background()), test.ShouldBeNil)
}

//nolint:dupl
func BenchmarkDepthRotate(b *testing.B) {
	img, err := rimage.NewDepthMapFromFile(artifact.MustPath("rimage/board1.dat.gz"))
	test.That(b, err, test.ShouldBeNil)

	source := gostream.NewVideoSource(&videosource.StaticSource{DepthImg: img}, prop.Video{})
	cam, err := camera.NewFromSource(source, nil)
	test.That(b, err, test.ShouldBeNil)
	rs, err := newRotateTransform(cam, camera.DepthStream)
	test.That(b, err, test.ShouldBeNil)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		camera.ReadImage(context.Background(), rs)
	}
	test.That(b, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(b, source.Close(context.Background()), test.ShouldBeNil)
}
