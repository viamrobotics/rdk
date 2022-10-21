package videosource

import (
	"bytes"
	"context"
	"image"
	"io"
	"net/http"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

func checkMimeType(ctx context.Context, data []byte) (string, error) {
	detectedMimeType := http.DetectContentType(data)
	requestedMime := gostream.MIMETypeHint(ctx, "")
	actualMime, _ := utils.CheckLazyMIMEType(requestedMime)
	if actualMime == utils.MimeTypeRawRGBA {
		if detectedMimeType != utils.MimeTypeDefault {
			return "", errors.Errorf(
				"attempted to decode data using %s format as raw rgba data, "+
					" raw rgba data must be encoded with our custom header",
				detectedMimeType,
			)
		}
		formatMatch := bytes.Compare(data[:4], rimage.RGBABitmapMagicNumber)
		if formatMatch != 0 {
			return "", errors.New(
				"attempted to decode raw rgba data, but data was not encoded with the expected header format")
		}
	} else if actualMime != "" && actualMime != detectedMimeType {
		return "", errors.Errorf(
			"mime type requested (%q) for lazy decode not returned (got %q)",
			actualMime, detectedMimeType,
		)
	}
	// in the event a MIME type was not specified, we want to use the type in
	// which the data was originally encoded, or failing that, provide the same
	// default ("application/octet-stream") as other standard libraries
	if requestedMime == "" {
		golog.Global().Debugf(
			"no MIME type specified, defaulting to detected type %s", detectedMimeType)
		requestedMime = detectedMimeType
	}
	return requestedMime, nil
}

func prepReadFromURL(ctx context.Context, client http.Client, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func readyBytesFromURL(ctx context.Context, client http.Client, url string) ([]byte, error) {
	body, err := prepReadFromURL(ctx, client, url)
	if err != nil {
		return nil, err
	}

	defer func() {
		viamutils.UncheckedError(body.Close())
	}()
	return io.ReadAll(body)
}

func readColorURL(ctx context.Context, client http.Client, url string) (image.Image, error) {
	colorData, err := readyBytesFromURL(ctx, client, url)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't ready color url")
	}

	mimeType, err := checkMimeType(ctx, colorData)
	if err != nil {
		return nil, err
	}
	actualMime, isLazy := utils.CheckLazyMIMEType(mimeType)
	if isLazy {
		return rimage.NewLazyEncodedImage(colorData, actualMime), nil
	}

	img, err := rimage.DecodeImage(ctx, colorData, actualMime)
	if err != nil {
		return nil, err
	}
	return rimage.ConvertImage(img), nil
}

func readDepthURL(ctx context.Context, client http.Client, url string, immediate bool) (image.Image, error) {
	depthData, err := readyBytesFromURL(ctx, client, url)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't ready depth url")
	}
	mimeType, err := checkMimeType(ctx, depthData)
	if err != nil {
		return nil, err
	}
	actualMimeType, isLazy := utils.CheckLazyMIMEType(mimeType)
	if !immediate && isLazy {
		return rimage.NewLazyEncodedImage(depthData, actualMimeType), nil
	}
	img, err := rimage.DecodeImage(ctx, depthData, mimeType)
	if err != nil {
		return nil, err
	}
	return rimage.ConvertImageToDepthMap(ctx, img)
}
