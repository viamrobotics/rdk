package slam_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

// Creates a mock camera server that serves 'pattern' using the files in 'path'.
// Assumes the files are named 0.png, 1.png, 2.png, etc.
func getMockCameraServer(t *testing.T, path, pattern string) *http.ServeMux {
	t.Helper()

	var index uint64

	handle := func(w http.ResponseWriter, r *http.Request) {
		i := atomic.AddUint64(&index, 1) - 1
		path := artifact.MustPath(path + strconv.FormatUint(i, 10) + ".png")
		bytes, err := os.ReadFile(path)
		test.That(t, err, test.ShouldBeNil)
		w.Write(bytes)
	}

	router := http.NewServeMux()
	router.HandleFunc(pattern, handle)
	return router
}

// Creates a mock color camera for orbslam integration testing. The returned Server must be
// closed when the test is finished.
func getMockColorCamera(t *testing.T, logger golog.Logger) (camera.Camera, *httptest.Server) {
	t.Helper()

	router := getMockCameraServer(t, "slam/temp_mock_camera/color/", "/color.png")
	svr := httptest.NewServer(router)
	intrinsics := &transform.PinholeCameraIntrinsics{
		Width:  1280,
		Height: 720,
		Fx:     200,
		Fy:     200,
		Ppx:    100,
		Ppy:    100,
	}
	attrs := videosource.ServerAttrs{
		URL: svr.URL + "/color.png",
		AttrConfig: &camera.AttrConfig{
			CameraParameters: intrinsics,
			Stream:           "color",
		},
	}
	cam, err := videosource.NewServerSource(context.Background(), &attrs, logger)
	test.That(t, err, test.ShouldBeNil)
	return cam, svr
}

// Creates a mock depth camera for orbslam integration testing. The returned Server must be
// closed when the test is finished.
func getMockDepthCamera(t *testing.T, logger golog.Logger) (camera.Camera, *httptest.Server) {
	t.Helper()

	router := getMockCameraServer(t, "slam/temp_mock_camera/depth/", "/depth.png")
	svr := httptest.NewServer(router)
	attrs := videosource.ServerAttrs{
		URL: svr.URL + "/depth.png",
		AttrConfig: &camera.AttrConfig{
			Stream: "depth",
		},
	}
	cam, err := videosource.NewServerSource(context.Background(), &attrs, logger)
	test.That(t, err, test.ShouldBeNil)
	return cam, svr
}

func TestMockCameraServer(t *testing.T) {
	logger := golog.NewTestLogger(t)

	t.Run("Color camera", func(t *testing.T) {
		colorCam, colorSvr := getMockColorCamera(t, logger)
		defer colorSvr.Close()

		proj, err := colorCam.Projector(context.Background())
		test.That(t, err, test.ShouldBeNil)
		intrinsics, ok := proj.(*transform.PinholeCameraIntrinsics)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, intrinsics.CheckValid(), test.ShouldBeNil)

		for i := 0; i < 15; i++ {
			t.Run(fmt.Sprintf("Test color image %v", i), func(t *testing.T) {
				img, release, err := camera.ReadImage(
					gostream.WithMIMETypeHint(context.Background(), utils.WithLazyMIMEType(utils.MimeTypePNG)), colorCam)
				test.That(t, err, test.ShouldBeNil)
				defer release()
				lazyImg, ok := img.(*rimage.LazyEncodedImage)
				test.That(t, ok, test.ShouldBeTrue)
				test.That(t, lazyImg.MIMEType(), test.ShouldEqual, utils.MimeTypePNG)
				path := artifact.MustPath("slam/temp_mock_camera/color/" + strconv.Itoa(i) + ".png")
				bytes, err := os.ReadFile(path)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, lazyImg.RawData(), test.ShouldResemble, bytes)
			})
		}
	})

	t.Run("Depth camera", func(t *testing.T) {
		depthCam, depthSvr := getMockDepthCamera(t, logger)
		defer depthSvr.Close()
		for i := 0; i < 15; i++ {
			t.Run(fmt.Sprintf("Test depth image %v", i), func(t *testing.T) {
				img, release, err := camera.ReadImage(
					gostream.WithMIMETypeHint(context.Background(), utils.WithLazyMIMEType(utils.MimeTypePNG)), depthCam)
				test.That(t, err, test.ShouldBeNil)
				defer release()
				lazyImg, ok := img.(*rimage.LazyEncodedImage)
				test.That(t, ok, test.ShouldBeTrue)
				test.That(t, lazyImg.MIMEType(), test.ShouldEqual, utils.MimeTypePNG)
				path := artifact.MustPath("slam/temp_mock_camera/depth/" + strconv.Itoa(i) + ".png")
				bytes, err := os.ReadFile(path)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, lazyImg.RawData(), test.ShouldResemble, bytes)
			})
		}
	})
}
