package vision

import (
	"strings"
	"testing"
)

func TestWebcamSource(t *testing.T) {
	s, err := NewWebcamSource(0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	a, _, err := s.NextColorDepthPair()
	defer a.Close()

	if err != nil {
		t.Fatal(err)
	}

}

func TestHttpSource(t *testing.T) {
	s := NewHttpSourceIntelEliot("127.0.0.1:8181")

	a, b, err := s.NextColorDepthPair()
	if err != nil {
		if strings.Index(err.Error(), "dial tcp 127.0.0.1:8181: connect: connection refused") >= 0 {
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
