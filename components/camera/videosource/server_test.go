//go:build !no_media

package videosource

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/viamrobotics/gostream"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

func createTestRouter(t *testing.T) (*http.ServeMux, image.Image, []byte, image.Image) {
	t.Helper()
	// get color image
	colorPath := artifact.MustPath("rimage/board1_small.png")
	// get depth image
	depthPath := artifact.MustPath("rimage/board1_gray_small.png")
	// get color bytes
	colorBytes, err := os.ReadFile(colorPath) // get png bytes
	test.That(t, err, test.ShouldBeNil)
	// get depth bytes
	dataDepth, err := os.ReadFile(depthPath)
	test.That(t, err, test.ShouldBeNil)
	// create mock router
	handleColor := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "image/png")
		w.Write(colorBytes)
	}
	handleDepth := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "image/png")
		w.Write(dataDepth)
	}
	router := http.NewServeMux()
	// expected depth image from raw data
	depthDatPath := artifact.MustPath("rimage/board1_gray_small.png")
	expectedDepth, err := rimage.NewDepthMapFromFile(context.Background(), depthDatPath)
	test.That(t, err, test.ShouldBeNil)
	router.HandleFunc("/color.png", handleColor)
	router.HandleFunc("/depth.png", handleDepth)

	expectedColor, err := png.Decode(bytes.NewReader(colorBytes))
	test.That(t, err, test.ShouldBeNil)

	return router, expectedColor, colorBytes, expectedDepth
}

func TestServerSource(t *testing.T) {
	logger := golog.NewTestLogger(t)
	// intrinsics for the image
	intrinsics := &transform.PinholeCameraIntrinsics{
		Width:  128,
		Height: 72,
		Fx:     821.32642889,
		Fy:     821.68607359,
		Ppx:    49.495941428,
		Ppy:    37.070529534,
	}
	// create mock server
	router, expectedColor, expectedColorBytes, expectedDepth := createTestRouter(t)
	svr := httptest.NewServer(router)
	defer svr.Close()
	// create color camera
	conf := ServerConfig{
		URL:              svr.URL + "/color.png",
		CameraParameters: intrinsics,
		Stream:           "color",
	}
	cam, err := NewServerSource(context.Background(), &conf, logger)
	test.That(t, err, test.ShouldBeNil)
	// read from mock server to get color
	img, release, err := camera.ReadImage(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)
	defer release()

	imgDecode, _, err := image.Decode(bytes.NewReader(expectedColorBytes))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img, test.ShouldResemble, rimage.ConvertImage(imgDecode))

	lazyCtx := gostream.WithMIMETypeHint(context.Background(), utils.WithLazyMIMEType(utils.MimeTypePNG))
	img, release, err = camera.ReadImage(
		lazyCtx,
		cam,
	)
	test.That(t, err, test.ShouldBeNil)
	defer release()

	lazyPng := rimage.NewLazyEncodedImage(expectedColorBytes, utils.MimeTypePNG)
	test.That(t, img, test.ShouldResemble, lazyPng)

	stream, err := cam.Stream(lazyCtx)
	test.That(t, err, test.ShouldBeNil)

	img, release, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	defer release()
	test.That(t, stream.Close(context.Background()), test.ShouldBeNil)
	test.That(t, img, test.ShouldResemble, lazyPng)

	// Requests for lazy JPEG can return lazy PNG
	lazyJPEGCtx := gostream.WithMIMETypeHint(context.Background(), utils.WithLazyMIMEType(utils.MimeTypeJPEG))
	img, release, err = camera.ReadImage(
		lazyJPEGCtx,
		cam,
	)
	test.That(t, err, test.ShouldBeNil)
	defer release()
	test.That(t, img, test.ShouldResemble, lazyPng)

	img, release, err = camera.ReadImage(
		gostream.WithMIMETypeHint(context.Background(), utils.MimeTypePNG),
		cam,
	)
	test.That(t, err, test.ShouldBeNil)
	defer release()

	test.That(t, img, test.ShouldResemble, rimage.ConvertImage(expectedColor))

	img, release, err = camera.ReadImage(
		gostream.WithMIMETypeHint(context.Background(), ""),
		cam,
	)
	test.That(t, err, test.ShouldBeNil)
	defer release()

	test.That(t, img, test.ShouldResemble, rimage.ConvertImage(expectedColor))

	_, err = cam.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, cam.Close(context.Background()), test.ShouldBeNil)

	// create depth camera
	conf2 := ServerConfig{
		URL:              svr.URL + "/depth.png",
		CameraParameters: intrinsics,
		Stream:           "depth",
	}
	cam2, err := NewServerSource(context.Background(), &conf2, logger)
	test.That(t, err, test.ShouldBeNil)
	img2, release, err := camera.ReadImage(context.Background(), cam2)
	test.That(t, err, test.ShouldBeNil)
	defer release()
	test.That(t, img2, test.ShouldResemble, expectedDepth)
	pc, err := cam2.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 128*72)
	test.That(t, cam2.Close(context.Background()), test.ShouldBeNil)
}

func TestDualServerSource(t *testing.T) {
	logger := golog.NewTestLogger(t)
	router, _, expectedColorBytes, expectedDepth := createTestRouter(t)
	svr := httptest.NewServer(router)
	defer svr.Close()
	// intrinsics for the image
	intrinsics := &transform.PinholeCameraIntrinsics{
		Width:  128,
		Height: 72,
		Fx:     821.32642889,
		Fy:     821.68607359,
		Ppx:    49.495941428,
		Ppy:    37.070529534,
	}
	// create camera with a color stream
	conf1 := dualServerConfig{
		Color:            svr.URL + "/color.png",
		Depth:            svr.URL + "/depth.png",
		CameraParameters: intrinsics,
		Stream:           "color",
	}
	cam1, err := newDualServerSource(context.Background(), &conf1, logger)
	test.That(t, err, test.ShouldBeNil)
	// read from mock server to get color image
	img, release, err := camera.ReadImage(context.Background(), cam1)
	test.That(t, err, test.ShouldBeNil)
	defer release()

	imgDecode, _, err := image.Decode(bytes.NewReader(expectedColorBytes))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img, test.ShouldResemble, rimage.ConvertImage(imgDecode))

	pc1, err := cam1.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cam1.Close(context.Background()), test.ShouldBeNil)

	// create camera with a depth stream
	conf2 := dualServerConfig{
		Color:            svr.URL + "/color.png",
		Depth:            svr.URL + "/depth.png",
		CameraParameters: intrinsics,
		Stream:           "depth",
	}
	cam2, err := newDualServerSource(context.Background(), &conf2, logger)
	test.That(t, err, test.ShouldBeNil)
	// read from mock server to get depth image
	dm, releaseDm, err := camera.ReadImage(context.Background(), cam2)
	test.That(t, err, test.ShouldBeNil)
	defer releaseDm()
	test.That(t, dm, test.ShouldResemble, expectedDepth)
	pc2, err := cam2.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	// compare point clouds, should be the same
	test.That(t, pc2, test.ShouldResemble, pc1)
	test.That(t, cam2.Close(context.Background()), test.ShouldBeNil)
}

func TestServerError(t *testing.T) {
	logger := golog.NewTestLogger(t)
	//nolint:dogsled
	router, _, _, _ := createTestRouter(t)
	svr := httptest.NewServer(router)
	defer svr.Close()

	conf := ServerConfig{
		// we expect a 404 error of MIME type "text/plain"
		URL:    svr.URL + "/bad_path",
		Stream: "color",
	}

	cam, err := NewServerSource(context.Background(), &conf, logger)
	test.That(t, err, test.ShouldBeNil)

	lazyCtx := gostream.WithMIMETypeHint(context.Background(), utils.WithLazyMIMEType(utils.MimeTypeJPEG))
	img, release, err := camera.ReadImage(lazyCtx, cam)

	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, img, test.ShouldBeNil)
	test.That(t, release, test.ShouldBeNil)
}
