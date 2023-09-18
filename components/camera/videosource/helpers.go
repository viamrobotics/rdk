package videosource

import (
	"bytes"
	"context"
	"image"
	"io"
	"net/http"
	"strings"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/rdk/gostream"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

// getMIMETypeFromData uses context to determine a MIME type requested by a parent
// process and attempts to detect the MIME type of the data from its header.
// If no MIME type has been requested or there is a mismatch, it returns the one
// detected by http.DetectContentType.
func getMIMETypeFromData(ctx context.Context, data []byte, logger golog.Logger) (string, error) {
	detectedMimeType := http.DetectContentType(data)
	if !strings.Contains(detectedMimeType, "image") {
		return "", errors.Errorf("cannot decode image from MIME type '%s'", detectedMimeType)
	}

	requestedMime := gostream.MIMETypeHint(ctx, "")
	actualMime, isLazy := utils.CheckLazyMIMEType(requestedMime)
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
	} else if (actualMime != "") && (actualMime != detectedMimeType) {
		logger.Debugf(
			"mime type requested %s for decode was not detected format %s,"+
				" using detected format", actualMime, detectedMimeType,
		)
	}
	// in the event a MIME type was not specified, we want to use the type in
	// which the data was originally encoded, or failing that, provide the same
	// default ("application/octet-stream") as other standard libraries.
	if requestedMime == "" {
		logger.Debugf(
			"no MIME type specified, defaulting to detected type %s", detectedMimeType)
	}
	if isLazy {
		return utils.WithLazyMIMEType(detectedMimeType), nil
	}
	return detectedMimeType, nil
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

func readColorURL(ctx context.Context, client http.Client, url string, logger golog.Logger) (image.Image, error) {
	colorData, err := readyBytesFromURL(ctx, client, url)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't ready color url")
	}
	mimeType, err := getMIMETypeFromData(ctx, colorData, logger)
	if err != nil {
		return nil, err
	}
	img, err := rimage.DecodeImage(ctx, colorData, mimeType)
	if err != nil {
		return nil, err
	}
	if _, isLazy := utils.CheckLazyMIMEType(mimeType); isLazy {
		return img, nil
	}
	return rimage.ConvertImage(img), nil
}

func readDepthURL(ctx context.Context, client http.Client, url string, immediate bool, logger golog.Logger) (image.Image, error) {
	depthData, err := readyBytesFromURL(ctx, client, url)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't ready depth url")
	}
	mimeType, err := getMIMETypeFromData(ctx, depthData, logger)
	if err != nil {
		return nil, err
	}
	img, err := rimage.DecodeImage(ctx, depthData, mimeType)
	if err != nil {
		return nil, err
	}
	_, isLazy := utils.CheckLazyMIMEType(mimeType)
	if !immediate && isLazy {
		return img, nil
	}
	return rimage.ConvertImageToDepthMap(ctx, img)
}
