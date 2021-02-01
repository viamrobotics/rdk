package vision

import (
	"context"
	"testing"

	"gocv.io/x/gocv"
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

	vImg, err := NewImage(img)
	if err != nil {
		t.Fatal(err)
	}
	gocv.IMWrite("out/test_rotate_source.png", vImg.MatUnsafe())

	// TODO(erh): actually validate image
}
