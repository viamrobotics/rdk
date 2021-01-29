package utils

import (
	"runtime"
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

	a, err := s.NextMat()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
}
