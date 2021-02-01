package vision

import (
	"context"
	"testing"
)

func TestRotateSource(t *testing.T) {
	pc, err := NewPointCloud("chess/data/board1.png", "chess/data/board1.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	source := &StaticSource{pc}

	rs := &RotateImageDepthSource{source}

	img, _, err := rs.NextImageDepthPair(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	vImg := NewImage(img)
	vImg.WriteTo("out/test_rotate_source.png")

	// TODO(erh): actually validate image
}
