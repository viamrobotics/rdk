package camera_test

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	camerautils "go.viam.com/rdk/robot/web/stream/camera2"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func TestGetStreamableNamedImageFromCamera(t *testing.T) {
	sourceImg := image.NewRGBA(image.Rect(0, 0, 1, 1))
	unstreamableImg, err := camera.NamedImageFromImage(sourceImg, "unstreamable", "image/undefined")
	test.That(t, err, test.ShouldBeNil)
	streamableImg, err := camera.NamedImageFromImage(sourceImg, "streamable", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)

	t.Run("no images", func(t *testing.T) {
		cam := &inject.Camera{
			ImagesFunc: func(
				ctx context.Context,
				sourceNames []string,
				extra map[string]interface{},
			) ([]camera.NamedImage, resource.ResponseMetadata, error) {
				return []camera.NamedImage{}, resource.ResponseMetadata{}, nil
			},
		}
		_, err := camerautils.GetStreamableNamedImageFromCamera(context.Background(), cam)
		test.That(t, err, test.ShouldBeError, errors.New(`no images received for camera "::/"`))
	})

	t.Run("no streamable images", func(t *testing.T) {
		cam := &inject.Camera{
			ImagesFunc: func(
				ctx context.Context,
				sourceNames []string,
				extra map[string]interface{},
			) ([]camera.NamedImage, resource.ResponseMetadata, error) {
				return []camera.NamedImage{unstreamableImg}, resource.ResponseMetadata{}, nil
			},
		}
		_, err := camerautils.GetStreamableNamedImageFromCamera(context.Background(), cam)
		test.That(t, err, test.ShouldBeError, errors.New(`no images were found with a streamable mime type for camera "::/"`))
	})

	t.Run("one streamable image", func(t *testing.T) {
		cam := &inject.Camera{
			ImagesFunc: func(
				ctx context.Context,
				sourceNames []string,
				extra map[string]interface{},
			) ([]camera.NamedImage, resource.ResponseMetadata, error) {
				return []camera.NamedImage{streamableImg}, resource.ResponseMetadata{}, nil
			},
		}
		img, err := camerautils.GetStreamableNamedImageFromCamera(context.Background(), cam)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, img.SourceName, test.ShouldEqual, "streamable")
	})

	t.Run("first image is not streamable", func(t *testing.T) {
		cam := &inject.Camera{
			ImagesFunc: func(
				ctx context.Context,
				sourceNames []string,
				extra map[string]interface{},
			) ([]camera.NamedImage, resource.ResponseMetadata, error) {
				return []camera.NamedImage{unstreamableImg, streamableImg}, resource.ResponseMetadata{}, nil
			},
		}
		img, err := camerautils.GetStreamableNamedImageFromCamera(context.Background(), cam)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, img.SourceName, test.ShouldEqual, "streamable")
	})

	t.Run("camera Images returns an error", func(t *testing.T) {
		expectedErr := errors.New("camera error")
		cam := &inject.Camera{
			ImagesFunc: func(
				ctx context.Context,
				sourceNames []string,
				extra map[string]interface{},
			) ([]camera.NamedImage, resource.ResponseMetadata, error) {
				return nil, resource.ResponseMetadata{}, expectedErr
			},
		}
		_, err := camerautils.GetStreamableNamedImageFromCamera(context.Background(), cam)
		test.That(t, err, test.ShouldBeError, expectedErr)
	})
}

func TestVideoSourceFromCamera(t *testing.T) {
	sourceImg := image.NewRGBA(image.Rect(0, 0, 4, 4))
	namedImg, err := camera.NamedImageFromImage(sourceImg, "test", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)
	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			return []camera.NamedImage{namedImg}, resource.ResponseMetadata{}, nil
		},
	}
	vs, err := camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)

	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	diffVal, _, err := rimage.CompareImages(img, sourceImg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, diffVal, test.ShouldEqual, 0)
}

func TestVideoSourceFromCameraFailure(t *testing.T) {
	malformedNamedImage, err := camera.NamedImageFromBytes([]byte("not a valid image"), "source", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)
	malformedCam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			return []camera.NamedImage{malformedNamedImage}, resource.ResponseMetadata{}, nil
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), malformedCam)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, vs, test.ShouldNotBeNil)

	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	_, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New("could not decode image config: image: unknown format"))
}

func TestVideoSourceFromCameraWithNonsenseMimeType(t *testing.T) {
	sourceImg := image.NewRGBA(image.Rect(0, 0, 4, 4))
	namedImg, err := camera.NamedImageFromImage(sourceImg, "test", "image/undefined")
	test.That(t, err, test.ShouldBeNil)

	camWithNonsenseMimeType := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			return []camera.NamedImage{namedImg}, resource.ResponseMetadata{}, nil
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), camWithNonsenseMimeType)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, vs, test.ShouldNotBeNil)

	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	_, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New(`no images were found with a streamable mime type for camera "::/"`))
}

func TestVideoSourceFromCamera_SourceSelection(t *testing.T) {
	sourceImg := image.NewRGBA(image.Rect(0, 0, 4, 4))
	unstreamableImg, err := camera.NamedImageFromImage(sourceImg, "unstreamable", "image/undefined")
	test.That(t, err, test.ShouldBeNil)
	streamableImg, err := camera.NamedImageFromImage(sourceImg, "streamable", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			if len(sourceNames) == 0 {
				return []camera.NamedImage{unstreamableImg, streamableImg}, resource.ResponseMetadata{}, nil
			}
			if len(sourceNames) == 1 && sourceNames[0] == "streamable" {
				return []camera.NamedImage{streamableImg}, resource.ResponseMetadata{}, nil
			}
			return nil, resource.ResponseMetadata{}, fmt.Errorf("unexpected source filter: %v", sourceNames)
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)

	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	diffVal, _, err := rimage.CompareImages(img, sourceImg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, diffVal, test.ShouldEqual, 0)
}

func TestVideoSourceFromCamera_Recovery(t *testing.T) {
	sourceImg1 := image.NewRGBA(image.Rect(0, 0, 4, 4))
	sourceImg2 := image.NewRGBA(image.Rect(0, 0, 6, 6))

	goodNamedImage, err := camera.NamedImageFromImage(sourceImg1, "good", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)
	fallbackNamedImage, err := camera.NamedImageFromImage(sourceImg2, "fallback", utils.MimeTypeJPEG)
	test.That(t, err, test.ShouldBeNil)

	firstSourceFailed := false
	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			if len(sourceNames) == 0 { // GetStreamableNamedImageFromCamera call
				if !firstSourceFailed {
					return []camera.NamedImage{goodNamedImage}, resource.ResponseMetadata{}, nil
				}
				return []camera.NamedImage{fallbackNamedImage}, resource.ResponseMetadata{}, nil
			}

			// getImageBySourceName call
			if sourceNames[0] == "good" {
				if !firstSourceFailed {
					firstSourceFailed = true
					return []camera.NamedImage{goodNamedImage}, resource.ResponseMetadata{}, nil
				}
				return nil, resource.ResponseMetadata{}, errors.New("source 'good' is gone")
			}
			if sourceNames[0] == "fallback" {
				return []camera.NamedImage{fallbackNamedImage}, resource.ResponseMetadata{}, nil
			}
			return nil, resource.ResponseMetadata{}, fmt.Errorf("unknown source %q", sourceNames[0])
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)

	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// First image should be the good one
	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	diffVal, _, err := rimage.CompareImages(img, sourceImg1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, diffVal, test.ShouldEqual, 0)

	// Second call should fail, as the source is now gone.
	_, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New("source 'good' is gone"))

	// Third image should be the fallback one, because the state machine reset.
	img, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	diffVal, _, err = rimage.CompareImages(img, sourceImg2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, diffVal, test.ShouldEqual, 0)
}

func TestVideoSourceFromCamera_NoImages(t *testing.T) {
	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			return []camera.NamedImage{}, resource.ResponseMetadata{}, nil
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, vs, test.ShouldNotBeNil)

	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	_, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New(`no images received for camera "::/"`))
}

func TestVideoSourceFromCamera_ImagesError(t *testing.T) {
	expectedErr := errors.New("camera error")
	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			return nil, resource.ResponseMetadata{}, expectedErr
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, vs, test.ShouldNotBeNil)

	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	_, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeError, expectedErr)
}

func TestVideoSourceFromCamera_MultipleStreamableSources(t *testing.T) {
	imageGood1 := image.NewRGBA(image.Rect(0, 0, 4, 4))
	imageGood2 := image.NewRGBA(image.Rect(0, 0, 6, 6))
	namedA, err := camera.NamedImageFromImage(imageGood1, "good1", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)
	namedB, err := camera.NamedImageFromImage(imageGood2, "good2", utils.MimeTypeJPEG)
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			// When unfiltered, return both streamable sources. Selection should pick good1 first.
			if len(sourceNames) == 0 {
				return []camera.NamedImage{namedA, namedB}, resource.ResponseMetadata{}, nil
			}
			// When filtered to good1, return only good1
			if len(sourceNames) == 1 && sourceNames[0] == "good1" {
				return []camera.NamedImage{namedA}, resource.ResponseMetadata{}, nil
			}
			return nil, resource.ResponseMetadata{}, fmt.Errorf("unexpected source filter: %v", sourceNames)
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)
	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	diffVal, _, err := rimage.CompareImages(img, imageGood1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, diffVal, test.ShouldEqual, 0)
}

func TestVideoSourceFromCamera_NoStreamableSources(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	unstream1, err := camera.NamedImageFromImage(src, "bad1", "image/undefined")
	test.That(t, err, test.ShouldBeNil)
	unstream2, err := camera.NamedImageFromImage(src, "bad2", "image/undefined")
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			return []camera.NamedImage{unstream1, unstream2}, resource.ResponseMetadata{}, nil
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)
	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)
	_, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New(`no images were found with a streamable mime type for camera "::/"`))
}

func TestVideoSourceFromCamera_FilterNoImages(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	good, err := camera.NamedImageFromImage(src, "good", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)
	fallback, err := camera.NamedImageFromImage(src, "fallback", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)

	var firstServed bool
	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			if len(sourceNames) == 0 {
				// Initial selection / or post-reset unfiltered selection
				if !firstServed {
					return []camera.NamedImage{good, fallback}, resource.ResponseMetadata{}, nil
				}
				// After the filtered failure, put fallback first so recovery can succeed
				return []camera.NamedImage{fallback, good}, resource.ResponseMetadata{}, nil
			}
			// Filtered to "good": return no images to simulate empty filter result
			if len(sourceNames) == 1 && sourceNames[0] == "good" {
				firstServed = true
				return []camera.NamedImage{}, resource.ResponseMetadata{}, nil
			}
			return nil, resource.ResponseMetadata{}, fmt.Errorf("unexpected sequence: %v", sourceNames)
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)
	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)
	// First Next() corresponds to the first filtered read; expect failure
	_, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New("no images found for requested source name: good"))
	// Next Next() should recover and serve fallback
	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	diffVal, _, err := rimage.CompareImages(img, src)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, diffVal, test.ShouldEqual, 0)
}

func TestVideoSourceFromCamera_FilterMultipleImages(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	good, err := camera.NamedImageFromImage(src, "good", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			if len(sourceNames) == 0 {
				return []camera.NamedImage{good}, resource.ResponseMetadata{}, nil
			}
			if len(sourceNames) == 1 && sourceNames[0] == "good" {
				// Return multiple images even though a single source was requested
				// This simulates older camera APIs that don't support filtering
				return []camera.NamedImage{good, good}, resource.ResponseMetadata{}, nil
			}
			return nil, resource.ResponseMetadata{}, fmt.Errorf("unexpected source filter: %v", sourceNames)
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)
	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)
	// First Next() should succeed, using the first image from the multiple returned images
	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	diffVal, _, err := rimage.CompareImages(img, src)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, diffVal, test.ShouldEqual, 0)
}

func TestVideoSourceFromCamera_FilterMultipleImages_NoMatchingSource(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img1, err := camera.NamedImageFromImage(src, "source1", utils.MimeTypeRawRGBA)
	test.That(t, err, test.ShouldBeNil)
	img2, err := camera.NamedImageFromImage(src, "source2", utils.MimeTypeRawRGBA)
	test.That(t, err, test.ShouldBeNil)

	var erroredOnce bool
	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			// Initial unfiltered call returns two options. The stream will select "source1".
			if len(sourceNames) == 0 {
				if !erroredOnce {
					return []camera.NamedImage{img1, img2}, resource.ResponseMetadata{}, nil
				}
				// For recovery, only return the second option.
				return []camera.NamedImage{img2}, resource.ResponseMetadata{}, nil
			}

			// A filtered call for "source1" will return two images that don't match, triggering an error.
			if len(sourceNames) == 1 && sourceNames[0] == "source1" {
				erroredOnce = true
				return []camera.NamedImage{img2, img2}, resource.ResponseMetadata{}, nil
			}

			// The filtered call for "source2" is used for a successful recovery.
			if len(sourceNames) == 1 && sourceNames[0] == "source2" {
				return []camera.NamedImage{img2}, resource.ResponseMetadata{}, nil
			}
			return nil, resource.ResponseMetadata{}, fmt.Errorf("unexpected source filter: %v", sourceNames)
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)
	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// The first call to `VideoSourceFromCamera` will select "source1". The first call to `stream.Next()`
	// will then request "source1" and receive two "source2" images, causing an error.
	_, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeError,
		errors.New(`no matching source name found for multiple returned images: requested "source1", got ["source2" "source2"]`))

	// On the next call, the stream should recover by performing an unfiltered images call.
	// The mock will return only the second image, and the stream should succeed.
	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	diffVal, _, err := rimage.CompareImages(img, src)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, diffVal, test.ShouldEqual, 0)

	// Subsequent calls should also succeed.
	img, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	diffVal, _, err = rimage.CompareImages(img, src)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, diffVal, test.ShouldEqual, 0)
}

func TestVideoSourceFromCamera_LazyDecodeConfigError(t *testing.T) {
	malformedImage := rimage.NewLazyEncodedImage(
		[]byte("not a valid image"),
		utils.MimeTypePNG,
	)

	namedImg, err := camera.NamedImageFromImage(malformedImage, "lazy-image", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			return []camera.NamedImage{namedImg}, resource.ResponseMetadata{}, nil
		},
	}

	_, err = camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)
}

func TestVideoSourceFromCamera_InvalidImageFirst_ThenValidAlsoAvailable(t *testing.T) {
	validImg := image.NewRGBA(image.Rect(0, 0, 4, 4))
	invalidBytes := []byte("not a valid image")
	invalidNamed, err := camera.NamedImageFromBytes(invalidBytes, "bad", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)
	validNamed, err := camera.NamedImageFromImage(validImg, "good", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			// Unfiltered call returns a valid combination, but first entry is invalid
			if len(sourceNames) == 0 {
				return []camera.NamedImage{invalidNamed, validNamed}, resource.ResponseMetadata{}, nil
			}
			// If filtered to bad, still bad
			if len(sourceNames) == 1 && sourceNames[0] == "bad" {
				return []camera.NamedImage{invalidNamed}, resource.ResponseMetadata{}, nil
			}
			// If filtered to good, return the good one
			if len(sourceNames) == 1 && sourceNames[0] == "good" {
				return []camera.NamedImage{validNamed}, resource.ResponseMetadata{}, nil
			}
			return nil, resource.ResponseMetadata{}, fmt.Errorf("unexpected filter: %v", sourceNames)
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)
	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)
	// First Next(): already failed, and unfiltered selection again chooses invalid first -> fail
	_, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New("could not decode image config: image: unknown format"))
	// Second Next(): still fails because selection continues to prioritize invalid-first combination
	_, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New("could not decode image config: image: unknown format"))
}

func TestVideoSourceFromCamera_FilterMismatchedSourceName(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	good, err := camera.NamedImageFromImage(src, "good", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)
	mismatched, err := camera.NamedImageFromImage(src, "bad", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)

	var askedOnce bool
	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			if len(sourceNames) == 0 {
				return []camera.NamedImage{good}, resource.ResponseMetadata{}, nil
			}
			if len(sourceNames) == 1 && sourceNames[0] == "good" {
				if !askedOnce {
					askedOnce = true
					// Return a single image with a different source name than requested
					return []camera.NamedImage{mismatched}, resource.ResponseMetadata{}, nil
				}
				// After reset, success
				return []camera.NamedImage{good}, resource.ResponseMetadata{}, nil
			}
			return nil, resource.ResponseMetadata{}, fmt.Errorf("unexpected filter: %v", sourceNames)
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)
	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)
	// First Next(): filtered mismatch should fail
	_, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeError, errors.New(`mismatched source name: requested "good", got "bad"`))
	// Second Next(): should recover and deliver the correct image
	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	diffVal, _, err := rimage.CompareImages(img, src)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, diffVal, test.ShouldEqual, 0)
}

func TestVideoSourceFromCamera_OddDimensionsCropped(t *testing.T) {
	// Create an image with odd dimensions
	oddImg := image.NewRGBA(image.Rect(0, 0, 3, 3))

	namedImg, err := camera.NamedImageFromImage(oddImg, "test", utils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)

	cam := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			sourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			return []camera.NamedImage{namedImg}, resource.ResponseMetadata{}, nil
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)

	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	streamedImg, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// Verify dimensions were cropped from 3x3 to 2x2 for x264 compatibility
	test.That(t, streamedImg.Bounds(), test.ShouldResemble, image.Rect(0, 0, 2, 2))
}

// TODO(https://viam.atlassian.net/browse/RSDK-11726): Remove this test.
func TestGetImagesInStreamServerEnvVar(t *testing.T) {
	ogVal, ok := os.LookupEnv(utils.GetImagesInStreamServerEnvVar)
	if ok {
		defer os.Setenv(utils.GetImagesInStreamServerEnvVar, ogVal)
	} else {
		defer os.Unsetenv(utils.GetImagesInStreamServerEnvVar)
	}

	t.Run("when env var is set to true, returns true", func(t *testing.T) {
		os.Setenv(utils.GetImagesInStreamServerEnvVar, "true")
		test.That(t, utils.GetImagesInStreamServer(), test.ShouldBeTrue)
	})

	t.Run("when env var is not set, returns false", func(t *testing.T) {
		os.Unsetenv(utils.GetImagesInStreamServerEnvVar)
		test.That(t, utils.GetImagesInStreamServer(), test.ShouldBeFalse)
	})
}
