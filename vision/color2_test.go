package vision

import (
	"image"
	"os"
	"testing"

	"gocv.io/x/gocv"

	"github.com/muesli/clusters"
	"github.com/muesli/kmeans"
)

func hsvfrom(point clusters.Coordinates) HSV {
	return HSV{point[0], point[1], point[2]}
}

type HSVObservation struct {
	hsv HSV
}

func (o HSVObservation) Coordinates() clusters.Coordinates {
	return clusters.Coordinates{o.hsv.H, o.hsv.S, o.hsv.V}
}

func (o HSVObservation) Distance(point clusters.Coordinates) float64 {
	return o.hsv.Distance(hsvfrom(point))
}

func TestColorSegment1(t *testing.T) {
	img, err := NewImageFromFile("data/chess-segment1.png")
	if err != nil {
		t.Fatal(err)
	}

	all := []clusters.Observation{}

	for x := 0; x < img.Width(); x++ {
		for y := 0; y < img.Height(); y++ {
			c := img.ColorHSV(image.Point{x, y})
			all = append(all, HSVObservation{c})
		}
	}

	km := kmeans.New()

	clusters, err := km.Partition(all, 4)
	if err != nil {
		t.Fatal(err)
	}

	diffs := ColorDiffs{}

	for x, c := range clusters {
		a := hsvfrom(c.Center)
		for y := x + 1; y < len(clusters); y++ {
			if x == y {
				continue
			}
			b := hsvfrom(clusters[y].Center)

			diff := a.Distance(b)
			diffs = append(diffs, ColorDiff{a, b, diff})
		}
	}

	os.MkdirAll("out", 0775)

	err = diffs.writeTo("out/foo.html")
	if err != nil {
		t.Fatal(err)
	}

	out := gocv.NewMatWithSize(img.Rows(), img.Cols(), gocv.MatTypeCV8UC3)
	out2, err := NewImage(out)
	if err != nil {
		t.Fatal(err)
	}

	for x := 0; x < img.Width(); x++ {
		for y := 0; y < img.Height(); y++ {
			c := img.ColorHSV(image.Point{x, y})
			cc := clusters.Nearest(HSVObservation{c})
			ccc := hsvfrom(clusters[cc].Center)
			out2.SetHSV(image.Point{x, y}, ccc)
		}
	}

	gocv.IMWrite("out/foo.png", out)

}
