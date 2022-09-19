package slam_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/videosource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

var numImages uint64 = 15

// Creates a mock camera server that serves the color and depth images in slam/temp_mock_camera.
// The color and depth handles each signal to the returned channel when done.
func getMockCameraServer(t *testing.T) (*httptest.Server, <-chan int) {
	t.Helper()
	out := make(chan int, 2)
	var colorIndex uint64
	var depthIndex uint64

	colorHandle := func(w http.ResponseWriter, r *http.Request) {
		i := atomic.AddUint64(&colorIndex, 1) - 1
		t.Logf("Color handle called with image %v", i)
		if i >= numImages {
			t.Logf("Color handle no more images")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		path := artifact.MustPath("slam/temp_mock_camera/color/" + strconv.FormatUint(i, 10) + ".png")
		bytes, err := os.ReadFile(path)
		if err != nil {
			t.Logf("Unexpected error in color handle with image %v: %v", i, err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write(bytes)
		if i >= numImages-1 {
			t.Logf("Color handle reached end of images")
			out <- 1
		}
	}

	depthHandle := func(w http.ResponseWriter, r *http.Request) {
		i := atomic.AddUint64(&depthIndex, 1) - 1
		t.Logf("Depth handle called with image %v", i)
		if i >= numImages {
			t.Logf("Depth handle no more images")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		path := artifact.MustPath("slam/temp_mock_camera/depth/" + strconv.FormatUint(i, 10) + ".png")
		bytes, err := os.ReadFile(path)
		if err != nil {
			t.Logf("Unexpected error in depth handle with image %v: %v", i, err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write(bytes)
		if i >= numImages-1 {
			t.Logf("Depth handle reached end of images")
			out <- 1
		}
	}

	router := http.NewServeMux()
	router.HandleFunc("/color.png", colorHandle)
	router.HandleFunc("/depth.png", depthHandle)
	svr := httptest.NewServer(router)
	return svr, out
}

// Tests that the next image read from the camera is image i in the directory.
// If not, checks other images in the directory for a match.
func testImage(t *testing.T, cam camera.Camera, dir string, i int) {
	t.Helper()
	img, release, err := camera.ReadImage(
		gostream.WithMIMETypeHint(context.Background(), utils.WithLazyMIMEType(utils.MimeTypePNG)), cam)
	test.That(t, err, test.ShouldBeNil)
	defer release()
	lazyImg, ok := img.(*rimage.LazyEncodedImage)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, lazyImg.MIMEType(), test.ShouldEqual, utils.MimeTypePNG)
	path := artifact.MustPath(dir + strconv.Itoa(i) + ".png")
	expectedBytes, err := os.ReadFile(path)
	test.That(t, err, test.ShouldBeNil)
	if !reflect.DeepEqual(lazyImg.RawData(), expectedBytes) {
		t.Logf("Expected read from camera to match image %v in directory %v", i, dir)
		for j := 0; j < int(numImages); j++ {
			jPath := artifact.MustPath(dir + strconv.Itoa(j) + ".png")
			jBytes, err := os.ReadFile(jPath)
			test.That(t, err, test.ShouldBeNil)
			if reflect.DeepEqual(lazyImg.RawData(), jBytes) {
				t.Logf("Camera returned image %v instead of image %v from directory %v", j, i, dir)
			}
		}
		test.That(t, false, test.ShouldBeTrue)
	}
}

func TestMockCameraServer(t *testing.T) {
	logger := golog.NewTestLogger(t)
	svr, done := getMockCameraServer(t)
	defer svr.Close()

	// Create color camera
	intrinsics := &transform.PinholeCameraIntrinsics{
		Width:  1280,
		Height: 720,
		Fx:     200,
		Fy:     200,
		Ppx:    100,
		Ppy:    100,
	}
	colorAttrs := videosource.ServerAttrs{
		URL: svr.URL + "/color.png",
		AttrConfig: &camera.AttrConfig{
			CameraParameters: intrinsics,
			Stream:           "color",
		},
	}
	colorCam, err := videosource.NewServerSource(context.Background(), &colorAttrs, logger)
	test.That(t, err, test.ShouldBeNil)

	// Create depth camera
	depthAttrs := videosource.ServerAttrs{
		URL: svr.URL + "/depth.png",
		AttrConfig: &camera.AttrConfig{
			Stream: "depth",
		},
	}
	depthCam, err := videosource.NewServerSource(context.Background(), &depthAttrs, logger)
	test.That(t, err, test.ShouldBeNil)

	// Check color intrinsics
	proj, err := colorCam.Projector(context.Background())
	test.That(t, err, test.ShouldBeNil)
	intrinsics, ok := proj.(*transform.PinholeCameraIntrinsics)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, intrinsics.CheckValid(), test.ShouldBeNil)

	// Check images
	for i := 0; i < int(numImages); i++ {
		t.Logf("Test color image %v", i)
		testImage(t, colorCam, "slam/temp_mock_camera/color/", i)

		t.Logf("Test depth image %v", i)
		testImage(t, depthCam, "slam/temp_mock_camera/depth/", i)
	}

	// Both color and depth should be done.
	<-done
	<-done

	// Check error responses
	_, _, err = camera.ReadImage(
		gostream.WithMIMETypeHint(context.Background(), utils.WithLazyMIMEType(utils.MimeTypePNG)), colorCam)
	test.That(t, err, test.ShouldNotBeNil)
	_, _, err = camera.ReadImage(
		gostream.WithMIMETypeHint(context.Background(), utils.WithLazyMIMEType(utils.MimeTypePNG)), depthCam)
	test.That(t, err, test.ShouldNotBeNil)
}
