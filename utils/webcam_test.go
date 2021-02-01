package utils

import (
	"context"
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

	_, err = s.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}
