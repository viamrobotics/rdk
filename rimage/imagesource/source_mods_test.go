package imagesource

import (
	"context"
	"image"
	"io/ioutil"
	"testing"

	"go.viam.com/robotcore/artifact"
	"go.viam.com/robotcore/rimage"
)

var outDir string

func init() {
	var err error
	outDir, err = ioutil.TempDir("", "rimage_imagesource")
	if err != nil {
		panic(err)
	}
}

func TestRotateSource(t *testing.T) {
	pc, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), true)
	if err != nil {
		t.Fatal(err)
	}

	source := &StaticSource{pc}
	rs := &RotateImageDepthSource{source}

	rawImage, _, err := rs.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	err = rimage.WriteImageToFile(outDir+"/test_rotate_source.png", rawImage)
	if err != nil {
		t.Fatal(err)
	}

	img := rimage.ConvertImage(rawImage)

	for x := 0; x < pc.Color.Width(); x++ {
		p1 := image.Point{x, 0}
		p2 := image.Point{pc.Color.Width() - x - 1, pc.Color.Height() - 1}

		a := pc.Color.Get(p1)
		b := img.Get(p2)

		d := a.Distance(b)
		if d != 0 {
			t.Errorf("colors don't match %v %v", a, b)
		}

		d1 := pc.Depth.Get(p1)
		d2 := rawImage.(*rimage.ImageWithDepth).Depth.Get(p2)

		if d1 != d2 {
			t.Errorf("depth doesn't match %v %v", d1, d2)
		}
	}

}

func BenchmarkRotate(b *testing.B) {
	pc, err := rimage.NewImageWithDepth(artifact.MustPath("rimage/board1.png"), artifact.MustPath("rimage/board1.dat.gz"), true)
	if err != nil {
		b.Fatal(err)
	}

	source := &StaticSource{pc}
	rs := &RotateImageDepthSource{source}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		rs.Next(context.Background())
	}
}
