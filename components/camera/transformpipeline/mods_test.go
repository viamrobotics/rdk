package transformpipeline

import (
	"context"
	"image"
	"testing"

	"github.com/pion/mediadevices/pkg/prop"
	"github.com/viamrobotics/gostream"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/videosource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

func TestCrop(t *testing.T) {
	am := utils.AttributeMap{
		"x_min_px": 10,
		"y_min_px": 30,
		"x_max_px": 20,
		"y_max_px": 40,
	}
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1_small.png"))
	test.That(t, err, test.ShouldBeNil)
	dm, err := rimage.NewDepthMapFromFile(
		context.Background(), artifact.MustPath("rimage/board1_gray_small.png"))
	test.That(t, err, test.ShouldBeNil)

	// test depth source
	source := gostream.NewVideoSource(&videosource.StaticSource{DepthImg: dm}, prop.Video{})
	out, _, err := camera.ReadImage(context.Background(), source)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Bounds().Dx(), test.ShouldEqual, 128)
	test.That(t, out.Bounds().Dy(), test.ShouldEqual, 72)

	rs, stream, err := newCropTransform(context.Background(), source, camera.DepthStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.DepthStream)
	out, _, err = camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Bounds().Dx(), test.ShouldEqual, 10)
	test.That(t, out.Bounds().Dy(), test.ShouldEqual, 10)
	test.That(t, out, test.ShouldHaveSameTypeAs, &rimage.DepthMap{})
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(t, source.Close(context.Background()), test.ShouldBeNil)

	// test color source
	source = gostream.NewVideoSource(&videosource.StaticSource{ColorImg: img}, prop.Video{})
	out, _, err = camera.ReadImage(context.Background(), source)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Bounds().Dx(), test.ShouldEqual, 128)
	test.That(t, out.Bounds().Dy(), test.ShouldEqual, 72)

	// crop within bounds
	rs, stream, err = newCropTransform(context.Background(), source, camera.ColorStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.ColorStream)
	out, _, err = camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Bounds().Dx(), test.ShouldEqual, 10)
	test.That(t, out.Bounds().Dy(), test.ShouldEqual, 10)
	test.That(t, out, test.ShouldHaveSameTypeAs, &image.NRGBA{})
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)
	// crop has limits bigger than the image dimensions, but just takes the window
	am = utils.AttributeMap{"x_min_px": 127, "x_max_px": 150, "y_min_px": 71, "y_max_px": 110}
	rs, stream, err = newCropTransform(context.Background(), source, camera.ColorStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.ColorStream)
	out, _, err = camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Bounds().Dx(), test.ShouldEqual, 1)
	test.That(t, out.Bounds().Dy(), test.ShouldEqual, 1)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)

	//  error - crop limits are outside of original image
	am = utils.AttributeMap{"x_min_px": 1000, "x_max_px": 2000, "y_min_px": 300, "y_max_px": 400}
	rs, stream, err = newCropTransform(context.Background(), source, camera.ColorStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.ColorStream)
	_, _, err = camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cropped image to 0 pixels")
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)
	// error - empty attribute
	am = utils.AttributeMap{}
	_, _, err = newCropTransform(context.Background(), source, camera.ColorStream, am)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot crop image")
	// error - negative attributes
	am = utils.AttributeMap{"x_min_px": -4}
	_, _, err = newCropTransform(context.Background(), source, camera.ColorStream, am)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "negative number")

	// close the source
	test.That(t, source.Close(context.Background()), test.ShouldBeNil)
}

func TestResizeColor(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("rimage/board1_small.png"))
	test.That(t, err, test.ShouldBeNil)

	am := utils.AttributeMap{
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

	am := utils.AttributeMap{
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
	am := utils.AttributeMap{
		"angle_degs": 180,
	}
	rs, stream, err := newRotateTransform(context.Background(), source, camera.ColorStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.ColorStream)

	rawImage, _, err := camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)

	err = rimage.WriteImageToFile(t.TempDir()+"/test_rotate_color_source.png", rawImage)
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

	source = gostream.NewVideoSource(&videosource.StaticSource{ColorImg: img}, prop.Video{})
	am = utils.AttributeMap{
		"angle_degs": 90,
	}
	rs, stream, err = newRotateTransform(context.Background(), source, camera.ColorStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.ColorStream)

	rawImage, _, err = camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)

	err = rimage.WriteImageToFile(t.TempDir()+"/test_rotate_color_source.png", rawImage)
	test.That(t, err, test.ShouldBeNil)

	img2 = rimage.ConvertImage(rawImage)

	for x := 0; x < img.Width(); x++ {
		p1 := image.Point{X: x}
		p2 := image.Point{X: img2.Width() - 1, Y: x}

		a := img.Get(p1)
		b := img2.Get(p2)

		d := a.Distance(b)
		test.That(t, d, test.ShouldEqual, 0)
	}

	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(t, source.Close(context.Background()), test.ShouldBeNil)

	source = gostream.NewVideoSource(&videosource.StaticSource{ColorImg: img}, prop.Video{})
	am = utils.AttributeMap{
		"angle_degs": -90,
	}
	rs, stream, err = newRotateTransform(context.Background(), source, camera.ColorStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.ColorStream)

	rawImage, _, err = camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)

	err = rimage.WriteImageToFile(t.TempDir()+"/test_rotate_color_source.png", rawImage)
	test.That(t, err, test.ShouldBeNil)

	img2 = rimage.ConvertImage(rawImage)

	for x := 0; x < img.Width(); x++ {
		p1 := image.Point{X: x}
		p2 := image.Point{Y: img2.Height() - 1 - x}

		a := img.Get(p1)
		b := img2.Get(p2)

		d := a.Distance(b)
		test.That(t, d, test.ShouldEqual, 0)
	}

	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(t, source.Close(context.Background()), test.ShouldBeNil)

	source = gostream.NewVideoSource(&videosource.StaticSource{ColorImg: img}, prop.Video{})
	am = utils.AttributeMap{
		"angle_degs": 270,
	}
	rs, stream, err = newRotateTransform(context.Background(), source, camera.ColorStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.ColorStream)

	rawImage, _, err = camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)

	err = rimage.WriteImageToFile(t.TempDir()+"/test_rotate_color_source.png", rawImage)
	test.That(t, err, test.ShouldBeNil)

	img2 = rimage.ConvertImage(rawImage)

	for x := 0; x < img.Width(); x++ {
		p1 := image.Point{X: x}
		p2 := image.Point{Y: img2.Height() - 1 - x}

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
	am := utils.AttributeMap{
		"angle_degs": 180,
	}
	rs, stream, err := newRotateTransform(context.Background(), source, camera.DepthStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.DepthStream)

	rawImage, _, err := camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)

	err = rimage.WriteImageToFile(t.TempDir()+"/test_rotate_depth_source.png", rawImage)
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

	source = gostream.NewVideoSource(&videosource.StaticSource{DepthImg: pc}, prop.Video{})
	am = utils.AttributeMap{
		"angle_degs": 90,
	}
	rs, stream, err = newRotateTransform(context.Background(), source, camera.DepthStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.DepthStream)

	rawImage, _, err = camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)

	err = rimage.WriteImageToFile(t.TempDir()+"/test_rotate_depth_source.png", rawImage)
	test.That(t, err, test.ShouldBeNil)

	dm, err = rimage.ConvertImageToDepthMap(context.Background(), rawImage)
	test.That(t, err, test.ShouldBeNil)

	for x := 0; x < pc.Width(); x++ {
		p1 := image.Point{X: x}
		p2 := image.Point{X: dm.Width() - 1, Y: x}

		d1 := pc.Get(p1)
		d2 := dm.Get(p2)

		test.That(t, d1, test.ShouldEqual, d2)
	}

	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(t, source.Close(context.Background()), test.ShouldBeNil)

	source = gostream.NewVideoSource(&videosource.StaticSource{DepthImg: pc}, prop.Video{})
	am = utils.AttributeMap{
		"angle_degs": -90,
	}
	rs, stream, err = newRotateTransform(context.Background(), source, camera.DepthStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.DepthStream)

	rawImage, _, err = camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)

	err = rimage.WriteImageToFile(t.TempDir()+"/test_rotate_depth_source.png", rawImage)
	test.That(t, err, test.ShouldBeNil)

	dm, err = rimage.ConvertImageToDepthMap(context.Background(), rawImage)
	test.That(t, err, test.ShouldBeNil)

	for x := 0; x < pc.Width(); x++ {
		p1 := image.Point{X: x}
		p2 := image.Point{Y: dm.Height() - 1 - x}

		d1 := pc.Get(p1)
		d2 := dm.Get(p2)

		test.That(t, d1, test.ShouldEqual, d2)
	}

	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(t, source.Close(context.Background()), test.ShouldBeNil)

	source = gostream.NewVideoSource(&videosource.StaticSource{DepthImg: pc}, prop.Video{})
	am = utils.AttributeMap{
		"angle_degs": 270,
	}
	rs, stream, err = newRotateTransform(context.Background(), source, camera.DepthStream, am)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldEqual, camera.DepthStream)

	rawImage, _, err = camera.ReadImage(context.Background(), rs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, rs.Close(context.Background()), test.ShouldBeNil)

	err = rimage.WriteImageToFile(t.TempDir()+"/test_rotate_depth_source.png", rawImage)
	test.That(t, err, test.ShouldBeNil)

	dm, err = rimage.ConvertImageToDepthMap(context.Background(), rawImage)
	test.That(t, err, test.ShouldBeNil)

	for x := 0; x < pc.Width(); x++ {
		p1 := image.Point{X: x}
		p2 := image.Point{Y: dm.Height() - 1 - x}

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
	src, err := camera.WrapVideoSourceWithProjector(context.Background(), source, nil, camera.ColorStream)
	test.That(b, err, test.ShouldBeNil)
	am := utils.AttributeMap{
		"angle_degs": 180,
	}
	rs, stream, err := newRotateTransform(context.Background(), src, camera.ColorStream, am)
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
	src, err := camera.WrapVideoSourceWithProjector(context.Background(), source, nil, camera.DepthStream)
	test.That(b, err, test.ShouldBeNil)
	am := utils.AttributeMap{
		"angle_degs": 180,
	}
	rs, stream, err := newRotateTransform(context.Background(), src, camera.DepthStream, am)
	test.That(b, err, test.ShouldBeNil)
	test.That(b, stream, test.ShouldEqual, camera.DepthStream)

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		camera.ReadImage(context.Background(), rs)
	}
	test.That(b, rs.Close(context.Background()), test.ShouldBeNil)
	test.That(b, source.Close(context.Background()), test.ShouldBeNil)
}
