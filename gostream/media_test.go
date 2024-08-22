package gostream

import (
	"context"
	_ "embed"
	"image"
	"image/color"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/test"

	"go.viam.com/rdk/rimage"
)

type imageSource struct {
	Images       []image.Image
	idx          int32 // Use atomic int32 for idx
	releaseCount int32 // Use atomic int32 for releaseCount
}

func (is *imageSource) Read(_ context.Context) (image.Image, func(), error) {
	currentIdx := atomic.AddInt32(&is.idx, 1) - 1

	if int(currentIdx) >= len(is.Images) {
		return nil, func() {}, nil
	}
	img := is.Images[currentIdx]
	release := func() {
		atomic.AddInt32(&is.releaseCount, 1)
	}
	return img, release, nil
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

	imgSource := &imageSource{Images: colors}
	videoSrc := NewVideoSource(imgSource, prop.Video{})
	// Test all images are returned in order.
	for i, expected := range colors {
		actual, release, err := ReadMedia(context.Background(), videoSrc)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actual, test.ShouldNotBeNil)
		test.That(t, actual, test.ShouldEqual, expected)

		// Call release and check if it sets the flag
		release()
		test.That(t, atomic.LoadInt32(&imgSource.releaseCount), test.ShouldEqual, int32(i+1))
	}
}

func TestImageComparison(t *testing.T) {
	// Image comparison should fail if two images are not the same
	imgSource := &imageSource{Images: []image.Image{createImage(rimage.Red)}}
	videoSrc := NewVideoSource(imgSource, prop.Video{})

	pink := createImage(rimage.Pink)
	red, release, err := ReadMedia(context.Background(), videoSrc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, red, test.ShouldNotEqual, pink)

	// Call release and check if it sets the flag
	release()
	test.That(t, atomic.LoadInt32(&imgSource.releaseCount), test.ShouldEqual, 1)
}

func TestMultipleConsumers(t *testing.T) {
	colors := []image.Image{
		createImage(rimage.Red),
		createImage(rimage.Blue),
		createImage(rimage.Green),
		createImage(rimage.Yellow),
		createImage(rimage.Purple),
		createImage(rimage.Cyan),
	}

	imgSource := &imageSource{Images: colors}

	numConsumers := 2
	var wg sync.WaitGroup
	wg.Add(numConsumers)

	for i := 0; i < numConsumers; i++ {
		go func() {
			videoSrc := NewVideoSource(imgSource, prop.Video{})
			defer func() {
				videoSrc.Close(context.Background())
				wg.Done()
			}()
			for j := 0; j < len(colors)/numConsumers; j++ {
				actual, release, err := ReadMedia(context.Background(), videoSrc)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, actual, test.ShouldNotBeNil)

				release()
			}
		}()
	}

	wg.Wait()
	test.That(t, atomic.LoadInt32(&imgSource.releaseCount), test.ShouldEqual, int32(len(imgSource.Images)))
}
