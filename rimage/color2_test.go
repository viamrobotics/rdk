package rimage

import (
	"image"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/test"
	"go.viam.com/robotcore/artifact"
	"go.viam.com/robotcore/testutils"
)

func TestColorSegment1(t *testing.T) {
	img, err := NewImageFromFile(artifact.MustPath("rimage/chess-segment1.png"))
	test.That(t, err, test.ShouldBeNil)

	all := []Color{}

	for x := 0; x < img.Width(); x++ {
		for y := 0; y < img.Height(); y++ {
			c := img.Get(image.Point{x, y})
			all = append(all, c)
		}
	}

	clusters, err := ClusterHSV(all, 4)
	test.That(t, err, test.ShouldBeNil)

	diffs := ColorDiffs{}

	for x, a := range clusters {
		for y := x + 1; y < len(clusters); y++ {
			if x == y {
				continue
			}
			b := clusters[y]

			diffs.Add(a, b)
		}
	}

	outDir := testutils.TempDir(t, "", "rimage")
	golog.NewTestLogger(t).Debugf("out dir: %q", outDir)
	err = diffs.WriteTo(outDir + "/foo.html")
	test.That(t, err, test.ShouldBeNil)

	out := NewImage(img.Width(), img.Height())
	for x := 0; x < img.Width(); x++ {
		for y := 0; y < img.Height(); y++ {
			c := img.Get(image.Point{x, y})
			_, cc, _ := c.Closest(clusters)
			out.Set(image.Point{x, y}, cc)
		}
	}

	out.WriteTo(outDir + "/foo.png")
}
