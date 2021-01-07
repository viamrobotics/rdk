package vision

import (
	"testing"

	"gocv.io/x/gocv"
)

func TestRotateSource(t *testing.T) {
	pc, err := NewPointCloud("chess/data/board1.png", "chess/data/board1.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	source := &StaticSource{pc}

	rs := &RotateSource{source}

	m, _, err := rs.NextColorDepthPair()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	gocv.IMWrite("out/test_rotate_source.png", m)

	// TODO(erh): actually validate image
}
