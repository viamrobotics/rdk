package gostream

import (
	"context"
	_ "embed"
	"image"
	"image/color"
	"testing"

	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"

	"go.viam.com/rdk/rimage"
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

func createImage(c color.Color) image.Image {
	w, h := 640, 480
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestReadMedia(t *testing.T) {
	colors := []image.Image{
		createImage(rimage.Red),
		createImage(rimage.Blue),
		createImage(rimage.Green),
		createImage(rimage.Yellow),
		createImage(rimage.Purple),
		createImage(rimage.Cyan),
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
	imgSource.Images = []image.Image{createImage(rimage.Red)}
	videoSrc = NewVideoSource(&imgSource, prop.Video{})

	blue := createImage(rimage.Blue)
	red, _, err := ReadMedia(context.Background(), videoSrc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, red, test.ShouldNotEqual, blue)
}
