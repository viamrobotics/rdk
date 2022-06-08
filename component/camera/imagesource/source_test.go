package imagesource

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/rimage"
)

const debugImageSource = "VIAM_DEBUG"

func debugImageSourceOrSkip(t *testing.T) {
	t.Helper()
	imageSourceTest := os.Getenv(debugImageSource)
	if imageSourceTest == "" {
		t.Skipf("set environmental variable %q to run this test", debugImageSource)
	}
}

func doServerSourceTest(t *testing.T, s gostream.ImageSource) {
	t.Helper()
	a, _, err := s.Next(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "dial tcp 127.0.0.1:8181: connect: connection refused") {
			t.Skip()
			return
		}
		test.That(t, err, test.ShouldBeNil)
	}

	b := rimage.ConvertToImageWithDepth(a).Depth

	bounds := a.Bounds()
	test.That(t, bounds.Max.X == 640 || bounds.Max.X == 1280, test.ShouldBeTrue)
	test.That(t, b.Width() == 640 || b.Width() == 1280, test.ShouldBeTrue)
	test.That(t, bounds.Max.X, test.ShouldEqual, b.Width())
}

func TestDualServerSourceNoDepth(t *testing.T) {
	s := dualServerSource{ColorURL: "https://via.placeholder.com/350x150.jpg", DepthURL: ""}
	_, _, err := s.Next(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New("couldn't ready depth url: Get \"\": unsupported protocol scheme \"\""))
}

func TestDualServerSource(t *testing.T) {
	root := "127.0.0.1:8181"
	s := &dualServerSource{
		ColorURL:  fmt.Sprintf("http://%s/pic.ppm", root),
		DepthURL:  fmt.Sprintf("http://%s/depth.dat", root),
		isAligned: true,
	}
	defer s.Close()

	doServerSourceTest(t, s)
}

func TestServerSource(t *testing.T) {
	logger := golog.NewTestLogger(t)
	attrs := ServerAttrs{}
	attrs.AttrConfig = &camera.AttrConfig{}
	attrs.Host = "127.0.0.1"
	attrs.Port = 8181
	attrs.Stream = "both"
	s, err := NewServerSource(&attrs, logger)
	test.That(t, err, test.ShouldBeNil)
	doServerSourceTest(t, s)
}
