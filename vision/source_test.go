package vision

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestHTTPSourceNoDepth(t *testing.T) {
	s := HTTPSource{ColorURL: "http://placehold.it/120x120&text=image1", DepthURL: ""}
	_, _, err := s.NextImageDepthPair(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func doHTTPSourceTest(t *testing.T, s ImageDepthSource) {
	a, b, err := s.NextImageDepthPair(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "dial tcp 127.0.0.1:8181: connect: connection refused") {
			t.Skip()
			return
		}
		t.Fatal(err)
	}

	bounds := a.Bounds()
	if bounds.Max.X != 640 && bounds.Max.X != 1280 {
		t.Errorf("color columns wrong: %d", bounds.Max.X)
	}

	if b.Cols() != 640 && b.Cols() != 1280 {
		t.Errorf("depth columns wrong: %d", b.Cols())
	}

	if bounds.Max.X != b.Cols() {
		t.Errorf("color and depth don't match")
	}
}

func TestHTTPSource(t *testing.T) {
	root := "127.0.0.1:8181"
	s := &HTTPSource{
		fmt.Sprintf("http://%s/pic.ppm", root),
		fmt.Sprintf("http://%s/depth.dat", root),
	}

	doHTTPSourceTest(t, s)
}

func TestHTTPSource2(t *testing.T) {
	s := NewIntelServerSource("127.0.0.1", 8181, nil)

	doHTTPSourceTest(t, s)
}
