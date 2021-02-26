package vision

import (
	"image"
	"os"
	"testing"

	"go.viam.com/robotcore/utils"
)

func TestColorSegment1(t *testing.T) {
	img, err := NewImageFromFile("data/chess-segment1.png")
	if err != nil {
		t.Fatal(err)
	}

	all := []utils.HSV{}

	for x := 0; x < img.Width(); x++ {
		for y := 0; y < img.Height(); y++ {
			c := img.ColorHSV(image.Point{x, y})
			all = append(all, c)
		}
	}

	clusters, err := ClusterHSV(all, 4)
	if err != nil {
		t.Fatal(err)
	}

	diffs := utils.ColorDiffs{}

	for x, a := range clusters {
		for y := x + 1; y < len(clusters); y++ {
			if x == y {
				continue
			}
			b := clusters[y]

			diff := a.Distance(b)
			diffs = append(diffs, utils.ColorDiff{a, b, diff})
		}
	}

	os.MkdirAll("out", 0775)

	err = diffs.WriteTo("out/foo.html")
	if err != nil {
		t.Fatal(err)
	}

	out := NewImage(img.Image())
	for x := 0; x < img.Width(); x++ {
		for y := 0; y < img.Height(); y++ {
			c := img.ColorHSV(image.Point{x, y})
			_, cc, _ := c.Closest(clusters)
			out.SetHSV(image.Point{x, y}, cc)
		}
	}

	out.WriteTo("out/foo.png")
}
