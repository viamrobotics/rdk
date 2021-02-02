package vision

import (
	"context"
	"image"
	"testing"

	"github.com/viamrobotics/robotcore/utils"
)

func TestRotateSource(t *testing.T) {
	pc, err := NewPointCloud("chess/data/board1.png", "chess/data/board1.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	source := &StaticSource{pc}

	rs := &RotateImageDepthSource{source}

	rawImage, dm, err := rs.NextImageDepthPair(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	err = utils.WriteImageToFile("out/test_rotate_source.png", rawImage)
	if err != nil {
		t.Fatal(err)
	}

	img := NewImage(rawImage)

	for x := 0; x < pc.Color.Width(); x++ {
		p1 := image.Point{x, 0}
		p2 := image.Point{pc.Color.Width() - x - 1, pc.Color.Height() - 1}

		a := pc.Color.Color(p1)
		b := img.Color(p2)

		d := ColorDistance(a, b)
		if d != 0 {
			t.Errorf("colors don't match %v %v", a, b)
		}

		d1 := pc.Depth.Get(p1)
		d2 := dm.Get(p2)

		if d1 != d2 {
			t.Errorf("depth doesn't match %v %v", d1, d2)
		}
	}

}
