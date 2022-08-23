package imagesource

import (
	"context"
	"image"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
)

func createTestRouter(t *testing.T) (*http.ServeMux, image.Image, image.Image, []byte, []byte) {
	t.Helper()
	// get color image
	colorPath := artifact.MustPath("rimage/board1.png")
	expectedColor, err := rimage.NewImageFromFile(colorPath)
	test.That(t, err, test.ShouldBeNil)
	// get depth image
	depthPath := artifact.MustPath("rimage/board1_gray.png")
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
	depthDatPath := artifact.MustPath("rimage/board1.dat.gz")
	expectedDepth, err := rimage.NewDepthMapFromFile(depthDatPath)
	test.That(t, err, test.ShouldBeNil)
	router.HandleFunc("/color.png", handleColor)
	router.HandleFunc("/depth.png", handleDepth)
	return router, expectedColor, expectedDepth, colorBytes, dataDepth
}

func TestServerSource(t *testing.T) {
	logger := golog.NewTestLogger(t)
	// intrinsics for the image
	intrinsics := &transform.PinholeCameraIntrinsics{
		Width:  1024,
		Height: 768,
		Fx:     821.32642889,
		Fy:     821.68607359,
		Ppx:    494.95941428,
		Ppy:    370.70529534,
	}
	// create mock server
	router, expectedColor, expectedDepth, expectedColorBytes, expectedDepthBytes := createTestRouter(t)
	svr := httptest.NewServer(router)
	defer svr.Close()
	// create color camera
	attrs := ServerAttrs{
		URL: svr.URL + "/color.png",
		AttrConfig: &camera.AttrConfig{
			CameraParameters: intrinsics,
			Stream:           "color",
		},
	}
	cam, err := NewServerSource(context.Background(), &attrs, logger)
	test.That(t, err, test.ShouldBeNil)
	// read from mock server to get color
	img, release, err := cam.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	defer release()
	test.That(t, img, test.ShouldResemble, expectedColor)
	_, err = cam.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldNotBeNil)
	imgBytes, mimeType, _, _, err := cam.GetFrame(context.Background(), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, imgBytes, test.ShouldResemble, expectedColorBytes)
	test.That(t, mimeType, test.ShouldEqual, "image/png")
	// create depth camera
	attrs2 := ServerAttrs{
		URL: svr.URL + "/depth.png",
		AttrConfig: &camera.AttrConfig{
			CameraParameters: intrinsics,
			Stream:           "depth",
		},
	}
	cam2, err := NewServerSource(context.Background(), &attrs2, logger)
	test.That(t, err, test.ShouldBeNil)
	img2, release, err := cam2.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	defer release()
	test.That(t, img2, test.ShouldResemble, expectedDepth)
	pc, err := cam2.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 921600)
	depthBytes, depthMimeType, _, _, err := cam2.GetFrame(context.Background(), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, depthBytes, test.ShouldResemble, expectedDepthBytes)
	test.That(t, depthMimeType, test.ShouldEqual, "image/png")
}

func TestDualServerSource(t *testing.T) {
	router, expectedColor, expectedDepth, expectedColorBytes, expectedDepthBytes := createTestRouter(t)
	svr := httptest.NewServer(router)
	defer svr.Close()
	// intrinsics for the image
	intrinsics := &transform.PinholeCameraIntrinsics{
		Width:  1024,
		Height: 768,
		Fx:     821.32642889,
		Fy:     821.68607359,
		Ppx:    494.95941428,
		Ppy:    370.70529534,
	}
	// create camera with a color stream
	attrs1 := dualServerAttrs{
		Color: svr.URL + "/color.png",
		Depth: svr.URL + "/depth.png",
		AttrConfig: &camera.AttrConfig{
			CameraParameters: intrinsics,
			Stream:           "color",
		},
	}
	cam1, err := newDualServerSource(context.Background(), &attrs1)
	test.That(t, err, test.ShouldBeNil)
	// read from mock server to get color image
	img, release, err := cam1.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	defer release()
	test.That(t, img, test.ShouldResemble, expectedColor)
	pc1, err := cam1.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	imgBytes, mimeType, _, _, err := cam1.GetFrame(context.Background(), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, imgBytes, test.ShouldResemble, expectedColorBytes)
	test.That(t, mimeType, test.ShouldEqual, "image/png")
	// create camera with a depth stream
	attrs2 := dualServerAttrs{
		Color: svr.URL + "/color.png",
		Depth: svr.URL + "/depth.png",
		AttrConfig: &camera.AttrConfig{
			CameraParameters: intrinsics,
			Stream:           "depth",
		},
	}
	cam2, err := newDualServerSource(context.Background(), &attrs2)
	test.That(t, err, test.ShouldBeNil)
	// read from mock server to get depth image
	dm, releaseDm, err := cam2.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	defer releaseDm()
	test.That(t, dm, test.ShouldResemble, expectedDepth)
	pc2, err := cam2.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	// compare point clouds, should be the same
	test.That(t, pc2, test.ShouldResemble, pc1)
	depthBytes, depthMimeType, _, _, err := cam2.GetFrame(context.Background(), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, depthBytes, test.ShouldResemble, expectedDepthBytes)
	test.That(t, depthMimeType, test.ShouldEqual, "image/png")
}
