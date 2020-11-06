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

	a, b, err := s.NextColorDepthPair()
	defer a.Close()
	defer b.Close()

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
	defer b.Close()

	if a.Cols() != 640 {
		t.Errorf("color columns wrong: %d", a.Cols())
	}

	if b.Cols() != 640 {
		t.Errorf("depth columns wrong: %d", b.Cols())
	}

}
