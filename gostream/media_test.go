package gostream

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"
)

type imageSource struct {
	Images []image.Image
	idx    int
}

// Returns the next image or nil if there are no more images left. This should never error.
func (is *imageSource) Read(_ context.Context) (image.Image, func(), error) {
	if is.idx >= len(is.Images) {
		return nil, func() {}, nil
	}
	img := is.Images[is.idx]
	is.idx++
	return img, func() {}, nil
}

func (is *imageSource) Close(_ context.Context) error {
	return nil
}

func pngToImage(t *testing.T, path string) image.Image {
	t.Helper()
	openBytes, err := os.ReadFile(path)
	test.That(t, err, test.ShouldBeNil)
	img, err := png.Decode(bytes.NewReader(openBytes))
	test.That(t, err, test.ShouldBeNil)
	return img
}

func TestReadMedia(t *testing.T) {
	colors := []image.Image{
		pngToImage(t, "data/red.png"),
		pngToImage(t, "data/blue.png"),
		pngToImage(t, "data/green.png"),
		pngToImage(t, "data/yellow.png"),
		pngToImage(t, "data/fuchsia.png"),
		pngToImage(t, "data/cyan.png"),
	}

	imgSource := imageSource{Images: colors}
	videoSrc := NewVideoSource(&imgSource, prop.Video{})
	// Test all images are returned in order.
	for i, expected := range colors {
		actual, _, err := ReadMedia(context.Background(), videoSrc)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actual, test.ShouldNotBeNil)
		for j, col := range colors {
			if col == expected {
				continue
			}
			if actual == col {
				t.Logf("did not expect actual color to equal other color at %d when expecting %d", j, i)
			}
		}
		test.That(t, actual, test.ShouldEqual, expected)
	}

	// Test image comparison can fail if two images are not the same
	imgSource.Images = []image.Image{pngToImage(t, "data/red.png")}
	videoSrc = NewVideoSource(&imgSource, prop.Video{})

	blue := pngToImage(t, "data/blue.png")
	red, _, err := ReadMedia(context.Background(), videoSrc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, red, test.ShouldNotEqual, blue)
}
