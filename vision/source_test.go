package vision

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

func TestWebcamSource(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("skipping webcam because on linux")
		return
	}

	s, err := NewWebcamSource(0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	a, _, err := s.NextColorDepthPair()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
}

func TestHTTPSourceNoDepth(t *testing.T) {
	s := HTTPSource{ColorURL: "http://www.echolabs.com/static/small.jpg", DepthURL: ""}
	a, _, err := s.NextColorDepthPair()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
}

func doHTTPSourceTest(t *testing.T, s MatSource) {
	a, b, err := s.NextColorDepthPair()
	if err != nil {
		if strings.Contains(err.Error(), "dial tcp 127.0.0.1:8181: connect: connection refused") {
			t.Skip()
			return
		}
		t.Fatal(err)
	}
	defer a.Close()

	if a.Cols() != 640 && a.Cols() != 1280 {
		t.Errorf("color columns wrong: %d", a.Cols())
	}

	if b.Cols() != 640 && b.Cols() != 1280 {
		t.Errorf("depth columns wrong: %d", b.Cols())
	}

	if a.Cols() != b.Cols() {
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
