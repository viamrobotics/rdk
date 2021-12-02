package imagesource

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"go.viam.com/core/rimage"

	"github.com/edaniels/gostream"
	"go.viam.com/test"
)

func TestHTTPSourceNoDepth(t *testing.T) {
	s := httpSource{ColorURL: "http://placehold.it/120x120&text=image1", DepthURL: ""}
	_, _, err := s.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func doHTTPSourceTest(t *testing.T, s gostream.ImageSource) {
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

func TestHTTPSource(t *testing.T) {
	root := "127.0.0.1:8181"
	s := &httpSource{
		ColorURL:  fmt.Sprintf("http://%s/pic.ppm", root),
		DepthURL:  fmt.Sprintf("http://%s/depth.dat", root),
		isAligned: true,
	}
	defer func() {
		test.That(t, s.Close(), test.ShouldBeNil)
	}()

	doHTTPSourceTest(t, s)
}

func TestHTTPSource2(t *testing.T) {
	s, err := NewIntelServerSource("127.0.0.1", 8181, nil)
	test.That(t, err, test.ShouldBeNil)
	doHTTPSourceTest(t, s)
}
