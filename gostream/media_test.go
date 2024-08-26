package gostream

import (
	"context"
	"image"
	"image/color"
	"sync"
	"testing"

	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"

	"go.viam.com/rdk/rimage"
)

// WrappedImage wraps an image.Image and includes a bool flag to track release status.
type WrappedImage struct {
	Image    image.Image
	released bool
	mu       sync.Mutex
	t        *testing.T
}

// Implement the image.Image interface for WrappedImage.
func (wi *WrappedImage) ColorModel() color.Model {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	if wi.released {
		wi.t.Fatalf("ColorModel method accessed after release")
	}
	return wi.Image.ColorModel()
}

func (wi *WrappedImage) Bounds() image.Rectangle {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	if wi.released {
		wi.t.Fatalf("Bounds method accessed after release")
	}
	return wi.Image.Bounds()
}

func (wi *WrappedImage) At(x, y int) color.Color {
	wi.mu.Lock()
	defer wi.mu.Unlock()
	if wi.released {
		wi.t.Fatalf("At method accessed after release")
	}
	return wi.Image.At(x, y)
}

type imageSource struct {
	WrappedImages []*WrappedImage
	idx           int
	mu            sync.Mutex
}

// Returns the next image or nil if there are no more images left. This should never error.
func (is *imageSource) Read(_ context.Context) (image.Image, func(), error) {
	is.mu.Lock()
	defer is.mu.Unlock()

	if is.idx >= len(is.WrappedImages) {
		return nil, func() {}, nil
	}
	wrappedImg := is.WrappedImages[is.idx]
	release := func() {
		wrappedImg.mu.Lock()
		wrappedImg.released = true
		wrappedImg.mu.Unlock()
	}

	is.idx++
	return wrappedImg, release, nil
}

func (is *imageSource) Close(_ context.Context) error {
	return nil
}

func createWrappedImage(t *testing.T, c color.Color) *WrappedImage {
	w, h := 640, 480
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, c)
		}
	}
	return &WrappedImage{Image: img, t: t}
}

func TestReadMedia(t *testing.T) {
	colors := []*WrappedImage{
		createWrappedImage(t, rimage.Red),
		createWrappedImage(t, rimage.Blue),
		createWrappedImage(t, rimage.Green),
		createWrappedImage(t, rimage.Yellow),
		createWrappedImage(t, rimage.Purple),
		createWrappedImage(t, rimage.Cyan),
	}

	imgSource := &imageSource{WrappedImages: colors}
	videoSrc := NewVideoSource(imgSource, prop.Video{})
	// Test all images are returned in order.
	for i, expected := range colors {
		actual, release, err := ReadMedia(context.Background(), videoSrc)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actual, test.ShouldNotBeNil)
		for j, color := range colors {
			if color == expected {
				continue
			}
			if actual == color {
				t.Logf("did not expect actual color to equal other color at %d when expecting %d", j, i)
			}
		}
		test.That(t, actual, test.ShouldEqual, expected)

		// Call release and check if it sets the flag
		release()
		w := actual.(*WrappedImage)
		test.That(t, w.released, test.ShouldBeTrue)
	}
}

// Test that image comparison should fail if two images are not the same.
func TestImageComparison(t *testing.T) {
	imgSource := &imageSource{WrappedImages: []*WrappedImage{createWrappedImage(t, rimage.Red)}}
	videoSrc := NewVideoSource(imgSource, prop.Video{})

	pink := createWrappedImage(t, rimage.Pink)
	red, release, err := ReadMedia(context.Background(), videoSrc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, red, test.ShouldNotEqual, pink)

	// Call release and check if it sets the flag
	release()
	w := red.(*WrappedImage)
	test.That(t, w.released, test.ShouldBeTrue)
}

// TestMultipleConsumers tests concurrent consumption of images from imageSource via stream.Next().
func TestStreamMultipleConsumers(t *testing.T) {
	colors := []*WrappedImage{
		createWrappedImage(t, rimage.Red),
		createWrappedImage(t, rimage.Blue),
		createWrappedImage(t, rimage.Green),
		createWrappedImage(t, rimage.Yellow),
		createWrappedImage(t, rimage.Purple),
		createWrappedImage(t, rimage.Cyan),
	}

	imgSource := &imageSource{WrappedImages: colors}
	videoSrc := NewVideoSource(imgSource, prop.Video{})
	stream, err := videoSrc.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	numConsumers := 3
	var wg sync.WaitGroup
	wg.Add(numConsumers)

	// Coordinates index accesses to images
	var mu sync.Mutex
	j := 0

	for i := 0; i < numConsumers; i++ {
		go func(consumerId int) {
			defer wg.Done()

			for {
				mu.Lock()
				if j >= len(colors) {
					mu.Unlock()
					break
				}
				currIndex := j
				j++
				mu.Unlock()

				t.Logf("Consumer %d is processing image %d\n", consumerId, currIndex)
				actual, release, err := stream.Next(context.Background())
				test.That(t, err, test.ShouldBeNil)
				test.That(t, actual, test.ShouldNotBeNil)
				test.That(t, release, test.ShouldNotBeNil)

				// Release the image and check if it was released
				t.Logf("Consumer %d releasing image %d\n", consumerId, currIndex+1)
				release()
			}
		}(i)
	}

	wg.Wait()
	videoSrc.Close(context.Background())

	// Verify that all images have been released
	for i, wrappedImg := range imgSource.WrappedImages {
		wrappedImg.mu.Lock()
		t.Logf("Image at index %d.", i)
		test.That(t, wrappedImg.released, test.ShouldBeTrue)
		wrappedImg.mu.Unlock()
	}
}
