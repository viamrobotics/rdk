package imagesource

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/config"
	"go.viam.com/core/rimage"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
)

func doServerSourceTest(t *testing.T, s gostream.ImageSource) {
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
	s := dualServerSource{ColorURL: "http://placehold.it/120x120&text=image1", DepthURL: ""}
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
	defer func() {
		test.That(t, s.Close(), test.ShouldBeNil)
	}()

	doServerSourceTest(t, s)
}

func TestIntelServerSource(t *testing.T) {
	logger := golog.NewTestLogger(t)
	attrs := config.AttributeMap{}
	s, err := NewIntelServerSource("127.0.0.1", 8181, attrs, logger)
	test.That(t, err, test.ShouldBeNil)
	doServerSourceTest(t, s)
}

func TestServerSource(t *testing.T) {
	logger := golog.NewTestLogger(t)
	attrs := config.AttributeMap{}
	s, err := NewServerSource("127.0.0.1", 8181, attrs, logger)
	test.That(t, err, test.ShouldBeNil)
	doServerSourceTest(t, s)
}
