package transformpipeline

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/videosource"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
)

var outDir string

func init() {
	var err error
	outDir, err = testutils.TempDir("", "camera_transformpipeline")
	if err != nil {
		panic(err)
	}
	golog.Global().Debugf("out dir: %q", outDir)
}

func TestResizeColor(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1_small.png"))
	test.That(t, err, test.ShouldBeNil)

	am := config.AttributeMap{
		"height_px": 20,
		"width_px":  30,
	}
	source := gostream.NewVideoSource(&videosource.StaticSource{ColorImg: img}, prop.Video{})
	out, _, err := camera.ReadImage(context.Background(), source)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Bounds().Dx(), test.ShouldEqual, 128)
	test.That(t, out.Bounds().Dy(), test.ShouldEqual, 72)

	rs, stream, err := newResizeTransform(context.Background(), source, camera.ColorStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.ColorStream)
	out, _, err = camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Bounds().Dx(), test.ShouldEqual, 30)
	test.That(t, out.Bounds().Dy(), test.ShouldEqual, 20)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(t, source.Close(context.Background()), test.ShouldBeNil)
}

func TestResizeDepth(t *testing.T) {
	img, err := rimage.NewDepthMapFromFile(
		context.Background(), artifact.MustPath("rimage/board1_gray_small.png"))
	test.That(t, err, test.ShouldBeNil)

	am := config.AttributeMap{
		"height_px": 40,
		"width_px":  60,
	}
	source := gostream.NewVideoSource(&videosource.StaticSource{DepthImg: img}, prop.Video{})
	out, _, err := camera.ReadImage(context.Background(), source)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Bounds().Dx(), test.ShouldEqual, 128)
	test.That(t, out.Bounds().Dy(), test.ShouldEqual, 72)

	rs, stream, err := newResizeTransform(context.Background(), source, camera.DepthStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.DepthStream)
	out, _, err = camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Bounds().Dx(), test.ShouldEqual, 60)
	test.That(t, out.Bounds().Dy(), test.ShouldEqual, 40)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(t, source.Close(context.Background()), test.ShouldBeNil)
}

func TestRotateColorSource(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1_small.png"))
	test.That(t, err, test.ShouldBeNil)

	source := gostream.NewVideoSource(&videosource.StaticSource{ColorImg: img}, prop.Video{})
	rs, stream, err := newRotateTransform(context.Background(), source, camera.ColorStream)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.ColorStream)

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
	pc, err := rimage.NewDepthMapFromFile(
		context.Background(), artifact.MustPath("rimage/board1_gray_small.png"))
	test.That(t, err, test.ShouldBeNil)

	source := gostream.NewVideoSource(&videosource.StaticSource{DepthImg: pc}, prop.Video{})
	rs, stream, err := newRotateTransform(context.Background(), source, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.DepthStream)

	rawImage, _, err := camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)

	err = rimage.WriteImageToFile(outDir+"/test_rotate_depth_source.png", rawImage)
	test.That(t, err, test.ShouldBeNil)

	dm, err := rimage.ConvertImageToDepthMap(context.Background(), rawImage)
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

func BenchmarkColorRotate(b *testing.B) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1.png"))
	test.That(b, err, test.ShouldBeNil)

	source := gostream.NewVideoSource(&videosource.StaticSource{ColorImg: img}, prop.Video{})
	cam, err := camera.NewFromSource(context.Background(), source, nil, camera.ColorStream)
	test.That(b, err, test.ShouldBeNil)
	rs, stream, err := newRotateTransform(context.Background(), cam, camera.ColorStream)
	test.That(b, err, test.ShouldBeNil)
	test.That(b, stream, test.ShouldEqual, camera.ColorStream)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		camera.ReadImage(context.Background(), rs)
	}
	test.That(b, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(b, source.Close(context.Background()), test.ShouldBeNil)
}

func BenchmarkDepthRotate(b *testing.B) {
	img, err := rimage.NewDepthMapFromFile(
		context.Background(), artifact.MustPath("rimage/board1.dat.gz"))
	test.That(b, err, test.ShouldBeNil)

	source := gostream.NewVideoSource(&videosource.StaticSource{DepthImg: img}, prop.Video{})
	cam, err := camera.NewFromSource(context.Background(), source, nil, camera.DepthStream)
	test.That(b, err, test.ShouldBeNil)
	rs, stream, err := newRotateTransform(context.Background(), cam, camera.DepthStream)
	test.That(b, err, test.ShouldBeNil)
	test.That(b, stream, test.ShouldEqual, camera.DepthStream)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		camera.ReadImage(context.Background(), rs)
	}
	test.That(b, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(b, source.Close(context.Background()), test.ShouldBeNil)
}
