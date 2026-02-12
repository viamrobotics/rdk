package utils_test

import (
	"bytes"
	"context"
	"image"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

func TestFormatStringToMimeType(t *testing.T) {
	testRect := image.Rect(0, 0, 100, 100)

	type testCase struct {
		mimeType       string
		createImage    func() image.Image
		expectedFormat string
	}

	testCases := []testCase{
		{
			mimeType: utils.MimeTypeRawRGBA,
			createImage: func() image.Image {
				return image.NewRGBA(testRect)
			},
			expectedFormat: "vnd.viam.rgba",
		},
		{
			mimeType: utils.MimeTypeRawDepth,
			createImage: func() image.Image {
				return image.NewGray16(testRect)
			},
			expectedFormat: "vnd.viam.dep",
		},
		{
			mimeType: utils.MimeTypeJPEG,
			createImage: func() image.Image {
				return image.NewRGBA(testRect)
			},
			expectedFormat: "jpeg",
		},
		{
			mimeType: utils.MimeTypePNG,
			createImage: func() image.Image {
				return image.NewRGBA(testRect)
			},
			expectedFormat: "png",
		},
		{
			mimeType: utils.MimeTypeQOI,
			createImage: func() image.Image {
				return image.NewRGBA(testRect)
			},
			expectedFormat: "qoi",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.mimeType, func(t *testing.T) {
			sourceImg := tc.createImage()
			imgBytes, err := rimage.EncodeImage(context.Background(), sourceImg, tc.mimeType)
			test.That(t, err, test.ShouldBeNil)

			_, format, err := image.DecodeConfig(bytes.NewReader(imgBytes))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, format, test.ShouldEqual, tc.expectedFormat)

			resultMimeType := utils.FormatStringToMimeType(format)
			test.That(t, resultMimeType, test.ShouldEqual, tc.mimeType)
		})
	}
}
