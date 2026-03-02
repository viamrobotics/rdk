package camera_test

import (
	"context"
	"fmt"
	"image"
	"reflect"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testCameraName    = "camera1"
	depthCameraName   = "camera_depth"
	failCameraName    = "camera2"
	missingCameraName = "camera3"
	source1Name       = "source1"
	source2Name       = "source2"
	source3Name       = "source3"
)

// TestImages asserts the core expected behavior of the Images API.
func TestImages(t *testing.T) {
	ctx := context.Background()
	t.Run("extra param", func(t *testing.T) {
		respImg := image.NewRGBA(image.Rect(0, 0, 10, 10))
		annotations1 := data.Annotations{BoundingBoxes: []data.BoundingBox{{Label: "annotation1"}}}

		cam := inject.NewCamera("extra_param_cam")
		cam.ImagesFunc = func(
			ctx context.Context,
			filterSourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			if len(extra) == 0 {
				return nil, resource.ResponseMetadata{}, fmt.Errorf("extra parameters required")
			}
			namedImg, err := camera.NamedImageFromImage(respImg, source1Name, rutils.MimeTypeRawRGBA, annotations1)
			if err != nil {
				return nil, resource.ResponseMetadata{}, err
			}
			return []camera.NamedImage{namedImg}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
		}

		t.Run("success with extra params", func(t *testing.T) {
			images, _, err := cam.Images(ctx, nil, map[string]interface{}{"param": "value"})
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(images), test.ShouldEqual, 1)
			img, err := images[0].Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img, respImg), test.ShouldBeTrue)
			test.That(t, images[0].SourceName, test.ShouldEqual, source1Name)
			test.That(t, images[0].Annotations, test.ShouldResemble, annotations1)
		})

		t.Run("error when no extra params", func(t *testing.T) {
			_, _, err := cam.Images(ctx, nil, nil)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldEqual, "extra parameters required")
		})
	})

	t.Run("filter source names", func(t *testing.T) {
		img1 := image.NewRGBA(image.Rect(0, 0, 10, 10))
		img2 := rimage.NewEmptyDepthMap(10, 10)
		img3 := image.NewNRGBA(image.Rect(0, 0, 30, 30))

		annotations1 := data.Annotations{BoundingBoxes: []data.BoundingBox{{Label: "object1"}}}
		annotations2 := data.Annotations{Classifications: []data.Classification{{Label: "object2"}}}
		annotations3 := data.Annotations{BoundingBoxes: []data.BoundingBox{{Label: "object3"}}}

		namedImg1, err := camera.NamedImageFromImage(img1, source1Name, rutils.MimeTypePNG, annotations1)
		test.That(t, err, test.ShouldBeNil)
		namedImg2, err := camera.NamedImageFromImage(img2, source2Name, rutils.MimeTypeRawDepth, annotations2)
		test.That(t, err, test.ShouldBeNil)
		namedImg3, err := camera.NamedImageFromImage(img3, source3Name, rutils.MimeTypeJPEG, annotations3)
		test.That(t, err, test.ShouldBeNil)

		allImgs := []camera.NamedImage{namedImg1, namedImg2, namedImg3}
		availableSources := map[string]camera.NamedImage{
			source1Name: namedImg1,
			source2Name: namedImg2,
			source3Name: namedImg3,
		}

		cam := inject.NewCamera("multiple_sources_cam")
		cam.ImagesFunc = func(
			ctx context.Context,
			filterSourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			if len(filterSourceNames) == 0 {
				return allImgs, resource.ResponseMetadata{}, nil
			}

			var result []camera.NamedImage
			for _, sourceName := range filterSourceNames {
				if img, ok := availableSources[sourceName]; ok {
					result = append(result, img)
				} else {
					return nil, resource.ResponseMetadata{}, fmt.Errorf("requested source name not found: %s", sourceName)
				}
			}
			return result, resource.ResponseMetadata{}, nil
		}

		t.Run("nil filter returns all sources", func(t *testing.T) {
			imgs, _, err := cam.Images(ctx, nil, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(imgs), test.ShouldEqual, 3)

			returnedSources := make(map[string]bool)
			for _, img := range imgs {
				returnedSources[img.SourceName] = true
			}
			test.That(t, len(returnedSources), test.ShouldEqual, 3)
			test.That(t, returnedSources[source1Name], test.ShouldBeTrue)
			test.That(t, returnedSources[source2Name], test.ShouldBeTrue)
			test.That(t, returnedSources[source3Name], test.ShouldBeTrue)
		})

		t.Run("empty filter returns all sources", func(t *testing.T) {
			imgs, _, err := cam.Images(ctx, []string{}, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(imgs), test.ShouldEqual, 3)

			returnedSources := make(map[string]bool)
			for _, img := range imgs {
				returnedSources[img.SourceName] = true
			}
			test.That(t, len(returnedSources), test.ShouldEqual, 3)
			test.That(t, returnedSources[source1Name], test.ShouldBeTrue)
			test.That(t, returnedSources[source2Name], test.ShouldBeTrue)
			test.That(t, returnedSources[source3Name], test.ShouldBeTrue)
		})

		t.Run("single valid source", func(t *testing.T) {
			imgs, _, err := cam.Images(ctx, []string{source2Name}, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(imgs), test.ShouldEqual, 1)
			test.That(t, imgs[0].SourceName, test.ShouldEqual, source2Name)
			test.That(t, imgs[0].MimeType(), test.ShouldEqual, rutils.MimeTypeRawDepth)
			test.That(t, imgs[0].Annotations, test.ShouldResemble, annotations2)
			img, err := imgs[0].Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img, img2), test.ShouldBeTrue)
		})

		t.Run("multiple valid sources", func(t *testing.T) {
			imgs, _, err := cam.Images(ctx, []string{source3Name, source1Name}, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(imgs), test.ShouldEqual, 2)
			returnedSources := map[string]bool{}
			for _, img := range imgs {
				returnedSources[img.SourceName] = true
			}
			test.That(t, returnedSources[source3Name], test.ShouldBeTrue)
			test.That(t, returnedSources[source1Name], test.ShouldBeTrue)

			test.That(t, imgs[0].MimeType(), test.ShouldEqual, rutils.MimeTypeJPEG)
			test.That(t, imgs[0].Annotations, test.ShouldResemble, annotations3)
			img, err := imgs[0].Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img, img3), test.ShouldBeTrue)

			test.That(t, imgs[1].MimeType(), test.ShouldEqual, rutils.MimeTypePNG)
			test.That(t, imgs[1].Annotations, test.ShouldResemble, annotations1)
			img, err = imgs[1].Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img, img1), test.ShouldBeTrue)
		})

		t.Run("single invalid source", func(t *testing.T) {
			_, _, err := cam.Images(ctx, []string{"invalid_source"}, nil)
			test.That(t, err, test.ShouldBeError)
			test.That(t, err.Error(), test.ShouldEqual, "requested source name not found: invalid_source")
		})

		t.Run("mix of valid and invalid sources", func(t *testing.T) {
			_, _, err := cam.Images(ctx, []string{source1Name, "invalid_source"}, nil)
			test.That(t, err, test.ShouldBeError)
			test.That(t, err.Error(), test.ShouldEqual, "requested source name not found: invalid_source")
		})
	})
}

func TestNamedImage(t *testing.T) {
	ctx := context.Background()
	testImg := image.NewRGBA(image.Rect(0, 0, 10, 10))
	testImgPNGBytes, err := rimage.EncodeImage(ctx, testImg, rutils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)
	testImgJPEGBytes, err := rimage.EncodeImage(ctx, testImg, rutils.MimeTypeJPEG)
	test.That(t, err, test.ShouldBeNil)
	badBytes := []byte("trust bro i'm an image ong")
	sourceName := "test_source"
	annotations := data.Annotations{
		BoundingBoxes: []data.BoundingBox{
			{Label: "object1"},
		},
	}
	t.Run("NamedImageFromBytes", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			ni, err := camera.NamedImageFromBytes(testImgPNGBytes, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ni.SourceName, test.ShouldEqual, sourceName)
			test.That(t, ni.MimeType(), test.ShouldEqual, rutils.MimeTypePNG)
			test.That(t, ni.Annotations, test.ShouldResemble, annotations)
		})
		t.Run("error on nil data", func(t *testing.T) {
			_, err := camera.NamedImageFromBytes(nil, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeError, errors.New("must provide image bytes to construct a named image from bytes"))
		})
		t.Run("auto-infer mime type from bytes", func(t *testing.T) {
			ni, err := camera.NamedImageFromBytes(testImgPNGBytes, sourceName, "", annotations)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ni.SourceName, test.ShouldEqual, sourceName)
			test.That(t, ni.MimeType(), test.ShouldEqual, rutils.MimeTypePNG)
			test.That(t, ni.Annotations, test.ShouldResemble, annotations)
		})
		t.Run("error when mime type is empty and bytes are invalid", func(t *testing.T) {
			invalidBytes := []byte("not an image")
			_, err := camera.NamedImageFromBytes(invalidBytes, sourceName, "", annotations)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "could not infer mime type")
		})
	})

	t.Run("NamedImageFromImage", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			ni, err := camera.NamedImageFromImage(testImg, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ni.SourceName, test.ShouldEqual, sourceName)
			test.That(t, ni.MimeType(), test.ShouldEqual, rutils.MimeTypePNG)
			test.That(t, ni.Annotations, test.ShouldResemble, annotations)
			img, err := ni.Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img, testImg), test.ShouldBeTrue)
		})
		t.Run("error on nil image", func(t *testing.T) {
			_, err := camera.NamedImageFromImage(nil, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeError, errors.New("must provide image to construct a named image from image"))
		})
		t.Run("defaults to JPEG when mime type is empty", func(t *testing.T) {
			ni, err := camera.NamedImageFromImage(testImg, sourceName, "", annotations)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ni.SourceName, test.ShouldEqual, sourceName)
			test.That(t, ni.MimeType(), test.ShouldEqual, rutils.MimeTypeJPEG)

			data, err := ni.Bytes(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, data, test.ShouldResemble, testImgJPEGBytes)
		})
	})

	t.Run("Image method", func(t *testing.T) {
		t.Run("when image is already populated, it should return the image and cache it", func(t *testing.T) {
			ni, err := camera.NamedImageFromImage(testImg, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)
			img, err := ni.Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img, testImg), test.ShouldBeTrue)

			// should return the same image instance
			img2, err := ni.Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, reflect.ValueOf(img).Pointer(), test.ShouldEqual, reflect.ValueOf(img2).Pointer())
		})

		t.Run("when only data is populated, it should decode the data and cache it", func(t *testing.T) {
			ni, err := camera.NamedImageFromBytes(testImgPNGBytes, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)

			// first call should decode
			img, err := ni.Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img, testImg), test.ShouldBeTrue)

			// second call should return cached image
			img2, err := ni.Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img2, testImg), test.ShouldBeTrue)
			test.That(t, reflect.ValueOf(img).Pointer(), test.ShouldEqual, reflect.ValueOf(img2).Pointer())
		})

		t.Run("error when neither image nor data is populated", func(t *testing.T) {
			var ni camera.NamedImage
			_, err := ni.Image(ctx)
			test.That(t, err, test.ShouldBeError, errors.New("no image or image bytes available"))
		})

		t.Run("error when data is invalid", func(t *testing.T) {
			ni, err := camera.NamedImageFromBytes(badBytes, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)
			_, err = ni.Image(ctx)
			test.That(t, err, test.ShouldBeError)
			test.That(t, err.Error(), test.ShouldEqual, "could not decode image config: image: unknown format")
		})

		t.Run("error when mime type mismatches and decode fails", func(t *testing.T) {
			ni, err := camera.NamedImageFromBytes(testImgJPEGBytes, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)
			_, err = ni.Image(ctx)
			test.That(t, err, test.ShouldBeError)
			test.That(t, err, test.ShouldWrap, camera.ErrMIMETypeBytesMismatch)
		})

		t.Run("error when decode fails for other reasons", func(t *testing.T) {
			corruptedPNGBytes := append([]byte(nil), testImgPNGBytes...)
			corruptedPNGBytes[len(corruptedPNGBytes)-5] = 0 // corrupt it

			ni, err := camera.NamedImageFromBytes(corruptedPNGBytes, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)
			_, err = ni.Image(ctx)
			test.That(t, err, test.ShouldBeError)
			test.That(t, err.Error(), test.ShouldEqual, "could not decode bytes into image.Image: png: invalid format: invalid checksum")
		})
	})

	t.Run("Bytes method", func(t *testing.T) {
		t.Run("when data is already populated, it should return the data and cache it", func(t *testing.T) {
			ni, err := camera.NamedImageFromBytes(testImgPNGBytes, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)
			data, err := ni.Bytes(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, data, test.ShouldResemble, testImgPNGBytes)

			// should return the same data instance
			data2, err := ni.Bytes(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, &data[0], test.ShouldEqual, &data2[0])
		})

		t.Run("when only image is populated, it should encode the image and cache it", func(t *testing.T) {
			ni, err := camera.NamedImageFromImage(testImg, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)

			// first call should encode
			data, err := ni.Bytes(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, data, test.ShouldResemble, testImgPNGBytes)

			// second call should return cached data
			data2, err := ni.Bytes(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, data2, test.ShouldResemble, testImgPNGBytes)
			test.That(t, &data[0], test.ShouldEqual, &data2[0])
		})

		t.Run("error when neither image nor data is populated", func(t *testing.T) {
			var ni camera.NamedImage
			_, err := ni.Bytes(ctx)
			test.That(t, err, test.ShouldBeError, errors.New("no image or image bytes available"))
		})

		t.Run("error when encoding fails", func(t *testing.T) {
			ni, err := camera.NamedImageFromImage(testImg, sourceName, "bad-mime-type", annotations)
			test.That(t, err, test.ShouldBeNil)
			_, err = ni.Bytes(ctx)
			test.That(t, err, test.ShouldBeError)
			test.That(t, err.Error(), test.ShouldEqual,
				`could not encode image with encoding bad-mime-type: do not know how to encode "bad-mime-type"`)
		})
	})

	t.Run("MimeType method", func(t *testing.T) {
		ni, err := camera.NamedImageFromBytes(testImgPNGBytes, sourceName, rutils.MimeTypePNG, annotations)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ni.MimeType(), test.ShouldEqual, rutils.MimeTypePNG)
	})

	t.Run("Annotations method", func(t *testing.T) {
		ni, err := camera.NamedImageFromBytes(testImgPNGBytes, sourceName, rutils.MimeTypePNG, annotations)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ni.Annotations, test.ShouldResemble, annotations)

		ni, err = camera.NamedImageFromBytes(testImgPNGBytes, sourceName, rutils.MimeTypePNG, data.Annotations{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ni.Annotations.Empty(), test.ShouldBeTrue)
	})
}
